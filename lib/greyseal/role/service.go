package role

import (
	"context"

	"go.uber.org/zap"

	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

var _ RoleService = (*roleService)(nil)

type roleService struct {
	roleRepo base.Repository[*greysealv1.Role]
	logger   *zap.Logger
}

func NewRoleService(
	roleRepo base.Repository[*greysealv1.Role],
	logger *zap.Logger,
) RoleService {
	return &roleService{
		roleRepo: roleRepo,
		logger:   logger,
	}
}

func (srv *roleService) List(con context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Role], error) {
	srv.logger.Info("listing roles")
	data, err := srv.roleRepo.List(con, lis.GetCursor(), uint(lis.GetCount()), nil)
	if err != nil {
		srv.logger.Error("failed to list roles", zap.Error(err))
	}
	return &base.ListGenericResponse[*greysealv1.Role]{
		Cursor: "",
		Count:  10,
		Data:   data,
	}, err
}

func (srv *roleService) Get(con context.Context, get base.GetRequest[*greysealv1.Role]) (base.GetResponse[*greysealv1.Role], error) {
	srv.logger.Info("getting role", zap.String("uuid", get.GetUuid()))
	data, err := srv.roleRepo.Get(con, get.GetUuid())
	if err != nil {
		srv.logger.Error("failed to get role", zap.String("uuid", get.GetUuid()), zap.Error(err))
	}
	return &base.GetGenericResponse[*greysealv1.Role]{
		Data: data,
	}, err
}

func (srv *roleService) Create(con context.Context, cre base.CreateRequest[*greysealv1.Role]) (base.CreateResponse[*greysealv1.Role], error) {
	srv.logger.Info("creating role", zap.String("name", cre.GetData().GetName()))
	err := srv.roleRepo.Create(con, cre.GetData())
	if err != nil {
		srv.logger.Error("failed to create role", zap.Error(err))
		return nil, err
	}
	srv.logger.Info("role created", zap.String("uuid", cre.GetData().GetUuid()))
	return &base.CreateGenericResponse[*greysealv1.Role]{
		Data: cre.GetData(),
	}, err
}

func (srv *roleService) Update(con context.Context, id string, data *greysealv1.Role) (*greysealv1.Role, error) {
	srv.logger.Info("updating role", zap.String("uuid", id))
	err := srv.roleRepo.Update(con, id, data)
	if err != nil {
		srv.logger.Error("failed to update role", zap.String("uuid", id), zap.Error(err))
		return nil, err
	}
	return data, nil
}

func (srv *roleService) Delete(con context.Context, id string) error {
	srv.logger.Info("deleting role", zap.String("uuid", id))
	err := srv.roleRepo.Delete(con, id)
	if err != nil {
		srv.logger.Error("failed to delete role", zap.String("uuid", id), zap.Error(err))
	}
	return err
}
