package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ ResourceService = (*resourceService)(nil)

// VectorIndexer stores chunk embeddings in the vector database.
type VectorIndexer interface {
	IndexChunks(ctx context.Context, resourceUUID string, chunks []string, embeddings [][]float32) error
}

// Embedder generates embeddings for a list of text documents.
type Embedder interface {
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}

// Scraper extracts plain text from a URL.
type Scraper interface {
	ScrapeText(ctx context.Context, url string) (string, error)
}

type resourceService struct {
	resourceRepo  base.Repository[*Resource]
	vectorIndexer VectorIndexer
	embedder      Embedder
	scraper       Scraper
}

func NewResourceService(
	repo base.Repository[*Resource],
	indexer VectorIndexer,
	embedder Embedder,
	scraper Scraper,
) ResourceService {
	return &resourceService{
		resourceRepo:  repo,
		vectorIndexer: indexer,
		embedder:      embedder,
		scraper:       scraper,
	}
}

func (srv *resourceService) List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*Resource], error) {
	data, err := srv.resourceRepo.List(ctx, lis.GetCursor(), uint(lis.GetCount()), nil)
	return &base.ListGenericResponse[*Resource]{
		Cursor: "",
		Count:  int32(len(data)),
		Data:   data,
	}, err
}

func (srv *resourceService) Get(ctx context.Context, get base.GetRequest[*Resource]) (base.GetResponse[*Resource], error) {
	data, err := srv.resourceRepo.Get(ctx, get.GetUuid())
	return &base.GetGenericResponse[*Resource]{Data: data}, err
}

func (srv *resourceService) Delete(ctx context.Context, id string) error {
	return srv.resourceRepo.Delete(ctx, id)
}

func (srv *resourceService) Ingest(ctx context.Context, r *Resource) (*Resource, error) {
	// 1. Assign UUID if empty, set CreatedAt
	if r.Uuid == "" {
		r.Uuid = uuid.New().String()
	}
	r.CreatedAt = timestamppb.New(time.Now())

	// 2. Save metadata to postgres
	if err := srv.resourceRepo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3. Fetch/load content based on source
	var text string
	switch r.Source {
	case Source_SOURCE_WEBSITE:
		if srv.scraper == nil {
			return nil, errors.New("scraper not configured")
		}
		scraped, err := srv.scraper.ScrapeText(ctx, r.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape URL: %w", err)
		}
		text = scraped

	case Source_SOURCE_TEXT:
		text = r.Path

	case Source_SOURCE_PDF:
		return nil, errors.New("PDF ingestion not yet supported")

	default:
		// Fallback: try fetching the path as a URL
		resp, err := http.Get(r.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch resource: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		text = string(body)
	}

	if text == "" {
		return nil, errors.New("empty content extracted from resource")
	}

	// 4. Chunk the text
	chunks := chunkText(text, 1000, 200)
	if len(chunks) == 0 {
		return nil, errors.New("no chunks generated from resource content")
	}

	// 5. Generate embeddings for each chunk
	if srv.embedder == nil {
		return nil, errors.New("embedder not configured")
	}
	embeddings, err := srv.embedder.EmbedDocuments(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// 6. Store chunks+embeddings in Qdrant
	if err := srv.vectorIndexer.IndexChunks(ctx, r.Uuid, chunks, embeddings); err != nil {
		return nil, fmt.Errorf("failed to index chunks: %w", err)
	}

	// 7. Set IndexedAt, update resource in postgres
	r.IndexedAt = timestamppb.New(time.Now())
	_ = srv.resourceRepo.Update(ctx, r.Uuid, r)

	return r, nil
}

// chunkText splits text into overlapping chunks of the given size and overlap.
func chunkText(text string, size, overlap int) []string {
	if size <= 0 || len(text) == 0 {
		return nil
	}
	runes := []rune(text)
	total := len(runes)
	if total <= size {
		return []string{string(runes)}
	}
	var chunks []string
	step := size - overlap
	if step <= 0 {
		step = 1
	}
	for start := 0; start < total; start += step {
		end := start + size
		if end > total {
			end = total
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == total {
			break
		}
	}
	return chunks
}
