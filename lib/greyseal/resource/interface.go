package resource

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type ResourceService interface {
	List(con context.Context, lis base.ListRequest) (base.ListResponse[*Resource], error)
	Get(con context.Context, get base.GetRequest[*Resource]) (base.GetResponse[*Resource], error)
	Create(con context.Context, cre base.CreateRequest[*Resource]) (base.CreateResponse[*Resource], error)
}
