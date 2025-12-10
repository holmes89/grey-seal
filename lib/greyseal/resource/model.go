package resource

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type ResourceRepository interface {
	Create(context.Context, *Resource) error
	Update(context.Context, string, *Resource) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*Resource, error)
	List(context.Context, string, uint, map[string][]any) ([]*Resource, error)
}

var _ base.Entity = (*Resource)(nil)
var _ base.Repository[*Resource] = (ResourceRepository)(nil)
