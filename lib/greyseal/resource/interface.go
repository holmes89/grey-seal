package resource

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

// ResourceService manages resource metadata persistence and triggers indexing.
type ResourceService interface {
	List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Resource], error)
	Get(ctx context.Context, get base.GetRequest[*greysealv1.Resource]) (base.GetResponse[*greysealv1.Resource], error)
	Ingest(ctx context.Context, data *greysealv1.Resource) (*greysealv1.Resource, error)
	Delete(ctx context.Context, id string) error
}

// Indexer publishes a resource into the encoding pipeline after it is persisted.
// For SOURCE_TEXT the content is published directly as a TextExtractedEvent.
// For SOURCE_WEBSITE and SOURCE_PDF the resource is enqueued for async content fetching.
type Indexer interface {
	Index(ctx context.Context, resource *greysealv1.Resource) error
}
