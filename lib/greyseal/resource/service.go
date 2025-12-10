package resource

import (
	"context"
	"fmt"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

var _ ResourceService = (*resourceService)(nil)
var _ base.Service[*Resource] = (*resourceService)(nil)

type resourceService struct {
	resourceRepo base.Repository[*Resource]
}

func NewResourceService(
	resourceRepo base.Repository[*Resource],
) ResourceService {
	return &resourceService{
		resourceRepo: resourceRepo,
	}
}

func (srv *resourceService) List(con context.Context, lis base.ListRequest) (base.ListResponse[*Resource], error) {
	data, err := srv.resourceRepo.List(con, lis.GetCursor(), uint(lis.GetCount()), nil)
	return &base.ListGenericResponse[*Resource]{
		Cursor: "",
		Count:  10,
		Data:   data,
	}, err
}

func (srv *resourceService) Get(con context.Context, get base.GetRequest[*Resource]) (base.GetResponse[*Resource], error) {
	fmt.Println("get resource", get.GetUuid())
	data, err := srv.resourceRepo.Get(con, get.GetUuid())
	return &base.GetGenericResponse[*Resource]{
		Data: data,
	}, err
}

func (srv *resourceService) Create(con context.Context, cre base.CreateRequest[*Resource]) (base.CreateResponse[*Resource], error) {
	fmt.Println("create resource", cre.GetData())
	err := srv.resourceRepo.Create(con, cre.GetData())
	if err != nil {
		return nil, err
	}
	return &base.CreateGenericResponse[*Resource]{
		Data: cre.GetData(),
	}, err
}
