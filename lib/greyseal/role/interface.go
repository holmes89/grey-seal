package role

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type RoleService interface {
	List(con context.Context, lis base.ListRequest) (base.ListResponse[*Role], error)
	Get(con context.Context, get base.GetRequest[*Role]) (base.GetResponse[*Role], error)
	Create(con context.Context, cre base.CreateRequest[*Role]) (base.CreateResponse[*Role], error)
	Update(con context.Context, id string, data *Role) (*Role, error)
	Delete(con context.Context, id string) error
}

