package role

import (
	"context"
	"fmt"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

var _ RoleService = (*roleService)(nil)

type roleService struct {
	roleRepo base.Repository[*greysealv1.Role]
}

func NewRoleService(
	roleRepo base.Repository[*greysealv1.Role],
) RoleService {
	return &roleService{
		roleRepo: roleRepo,
	}
}

func (srv *roleService) List(con context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Role], error) {
	data, err := srv.roleRepo.List(con, lis.GetCursor(), uint(lis.GetCount()), nil)
	return &base.ListGenericResponse[*greysealv1.Role]{
		Cursor: "",
		Count:  10,
		Data:   data,
	}, err
}

func (srv *roleService) Get(con context.Context, get base.GetRequest[*greysealv1.Role]) (base.GetResponse[*greysealv1.Role], error) {
	fmt.Println("get role", get.GetUuid())
	data, err := srv.roleRepo.Get(con, get.GetUuid())
	return &base.GetGenericResponse[*greysealv1.Role]{
		Data: data,
	}, err
}

func (srv *roleService) Create(con context.Context, cre base.CreateRequest[*greysealv1.Role]) (base.CreateResponse[*greysealv1.Role], error) {
	fmt.Println("create role", cre.GetData())
	err := srv.roleRepo.Create(con, cre.GetData())
	if err != nil {
		return nil, err
	}
	return &base.CreateGenericResponse[*greysealv1.Role]{
		Data: cre.GetData(),
	}, err
}

func (srv *roleService) Update(con context.Context, id string, data *greysealv1.Role) (*greysealv1.Role, error) {
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
