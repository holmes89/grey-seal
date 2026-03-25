package role

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type RoleRepository interface {
	Create(context.Context, *greysealv1.Role) error
	Update(context.Context, string, *greysealv1.Role) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*greysealv1.Role, error)
	List(context.Context, string, uint, map[string][]any) ([]*greysealv1.Role, error)
}

var _ base.Entity = (*greysealv1.Role)(nil)
var _ base.Repository[*greysealv1.Role] = (RoleRepository)(nil)
