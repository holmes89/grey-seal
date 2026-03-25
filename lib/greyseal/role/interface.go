package role

import (
	"context"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type RoleService interface {
	List(con context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Role], error)
	Get(con context.Context, get base.GetRequest[*greysealv1.Role]) (base.GetResponse[*greysealv1.Role], error)
	Create(con context.Context, cre base.CreateRequest[*greysealv1.Role]) (base.CreateResponse[*greysealv1.Role], error)
	Update(con context.Context, id string, data *greysealv1.Role) (*greysealv1.Role, error)
	Delete(con context.Context, id string) error
}
