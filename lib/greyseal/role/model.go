package role

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type RoleRepository interface {
	Create(context.Context, *Role) error
	Update(context.Context, string, *Role) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*Role, error)
	List(context.Context, string, uint, map[string][]any) ([]*Role, error)
}

var _ base.Entity = (*Role)(nil)
var _ base.Repository[*Role] = (RoleRepository)(nil)

