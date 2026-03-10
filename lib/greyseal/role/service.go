package role

import (
	"context"
	"fmt"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

var _ RoleService = (*roleService)(nil)

type roleService struct {
	roleRepo base.Repository[*Role]
}

func NewRoleService(
	roleRepo base.Repository[*Role],
) RoleService {
	return &roleService{
		roleRepo: roleRepo,
	}
}

func (srv *roleService) List(con context.Context, lis base.ListRequest) (base.ListResponse[*Role], error) {
	data, err := srv.roleRepo.List(con, lis.GetCursor(), uint(lis.GetCount()), nil)
	return &base.ListGenericResponse[*Role]{
		Cursor: "",
		Count:  10,
		Data:   data,
	}, err
}

func (srv *roleService) Get(con context.Context, get base.GetRequest[*Role]) (base.GetResponse[*Role], error) {
	fmt.Println("get role", get.GetUuid())
	data, err := srv.roleRepo.Get(con, get.GetUuid())
	return &base.GetGenericResponse[*Role]{
		Data: data,
	}, err
}

func (srv *roleService) Create(con context.Context, cre base.CreateRequest[*Role]) (base.CreateResponse[*Role], error) {
	fmt.Println("create role", cre.GetData())
	err := srv.roleRepo.Create(con, cre.GetData())
	if err != nil {
		return nil, err
	}
	return &base.CreateGenericResponse[*Role]{
		Data: cre.GetData(),
	}, err
}

func (srv *roleService) Update(con context.Context, id string, data *Role) (*Role, error) {
	fmt.Println("update role", id)
	err := srv.roleRepo.Update(con, id, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (srv *roleService) Delete(con context.Context, id string) error {
	fmt.Println("delete role", id)
	return srv.roleRepo.Delete(con, id)
}

