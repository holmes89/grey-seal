package resource

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type ResourceService interface {
	List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*Resource], error)
	Get(ctx context.Context, get base.GetRequest[*Resource]) (base.GetResponse[*Resource], error)
	Delete(ctx context.Context, id string) error

	// Ingest saves resource metadata and triggers chunking + embedding into the vector store.
	Ingest(ctx context.Context, r *Resource) (*Resource, error)
}
