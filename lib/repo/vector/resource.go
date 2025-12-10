package vector

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/holmes89/archaea/base"
	"github.com/holmes89/grey-seal/lib/greyseal/question"
	"github.com/holmes89/grey-seal/lib/repo"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/pgvector/pgvector-go"

	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"

	sq "github.com/Masterminds/squirrel"
)

var _ base.Repository[*Resource] = (*ResourceVectorRepo)(nil)

type Scraper interface {
	ScrapeContent(ctx context.Context, url string) (io.Reader, error)
}

type ResourceVectorRepo struct {
	base.Repository[*Resource]
	conn     *repo.Conn
	scraper  Scraper
	Embedder embeddings.Embedder
}

func NewResourceVectorRepo(
	repo base.Repository[*Resource],
	sc Scraper,
	conn *repo.Conn,
	embedder embeddings.Embedder) *ResourceVectorRepo {
	return &ResourceVectorRepo{
		Repository: repo,
		conn:       conn,
		scraper:    sc,
		Embedder:   embedder,
	}
}

func (r *ResourceVectorRepo) LoadWebsite(ctx context.Context, b *Resource) ([]schema.Document, error) {
	if b.Path == "" {
		return nil, errors.New("path must be set for website resource")
	}
	htmlContent, err := r.scraper.ScrapeContent(ctx, b.Path)
	if err != nil {
		fmt.Println("unable to scrape content", err)
		return nil, err
	}

	// 2. Load and parse HTML
	loader := documentloaders.NewHTML(htmlContent)
	return loader.Load(context.Background())
}

func (r *ResourceVectorRepo) Create(ctx context.Context, b *Resource) error {
	err := r.Repository.Create(ctx, b)
	if err != nil {
		fmt.Println("unable to save resource", err)
		return err
	}
	var docs []schema.Document
	switch b.Source {
	case Source_SOURCE_WEBSITE:
		docs, err = r.LoadWebsite(ctx, b)
	}
	if err != nil {
		return fmt.Errorf("unable to parse content: %w", err)
	}
	if len(docs) == 0 {
		return nil
	}
	// 3. Split into chunks
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(1000),
		textsplitter.WithChunkOverlap(200),
	)
	chunks, err := splitter.SplitText(docs[0].PageContent)
	if err != nil {
		fmt.Println("error chunking text", err)
		return err
	}
	// 4. Generate embeddings
	embeddings, err := r.Embedder.EmbedDocuments(context.Background(), chunks)
	if err != nil {
		fmt.Println("error embedding documents", err)
		return err
	}
	// 5. Store in pgvector
	for i, embedding := range embeddings {
		vect := pgvector.NewVector(embedding)
		// Insert into resource_chunks table
		_, err = sq.StatementBuilder.
			PlaceholderFormat(sq.Dollar).
			Insert("resource_embeddings").
			Columns("resource_uuid",
				"chunk_index",
				"content",
				"embedding").
			Values(
				b.Uuid,
				i,
				chunks[i],
				vect,
			).
			RunWith(r.conn.DB()).Exec()
		if err != nil {
			fmt.Println("error saving resource embedding", err)
			return err
		}
	}
	return nil
}

func (r *ResourceVectorRepo) Query(ctx context.Context, query string, limit int) ([]question.QueryResult, error) {
	// 1. Generate embedding for the query
	queryEmbedding, err := r.Embedder.EmbedQuery(ctx, query)
	if err != nil {
		fmt.Println("error embedding query", err)
		return nil, err
	}

	// 2. Convert to pgvector format
	queryVector := pgvector.NewVector(queryEmbedding)

	// 3. Query for similar chunks using cosine similarity
	rows, err := r.conn.DB().QueryContext(ctx, "SELECT resource_uuid, content from resource_embeddings ORDER BY embedding <=> $1 LIMIT $2", queryVector, limit)
	if err != nil {
		fmt.Println("error querying embeddings", err)
		return nil, err
	}
	defer rows.Close()

	// 4. Collect results
	var results []question.QueryResult
	for rows.Next() {
		var res question.QueryResult
		if err := rows.Scan(&res.ResourceUUID, &res.Content); err != nil {
			fmt.Println("error scanning row", err)
			return nil, err
		}
		results = append(results, res)
	}

	if err := rows.Err(); err != nil {
		fmt.Println("error iterating rows", err)
		return nil, err
	}

	return results, nil
}
