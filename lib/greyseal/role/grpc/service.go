package grpc

import (
	"context"
	"log"

	"connectrpc.com/connect"

	entity "github.com/holmes89/grey-seal/lib/greyseal/role"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
)

type RoleHandler struct {
	servicesconnect.UnimplementedRoleServiceHandler
	svc entity.RoleService
}

func NewRoleHandler(svc entity.RoleService) *RoleHandler {
	return &RoleHandler{svc: svc}
}

func (h *RoleHandler) CreateRole(ctx context.Context, req *connect.Request[services.CreateRoleRequest]) (*connect.Response[services.CreateRoleResponse], error) {
	result, err := h.svc.Create(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.CreateRoleResponse{Data: result.GetData()}), nil
}

func (h *RoleHandler) GetRole(ctx context.Context, req *connect.Request[services.GetRoleRequest]) (*connect.Response[services.GetRoleResponse], error) {
	result, err := h.svc.Get(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.GetRoleResponse{Data: result.GetData()}), nil
}

func (h *RoleHandler) ListRoles(ctx context.Context, req *connect.Request[services.ListRolesRequest]) (*connect.Response[services.ListRolesResponse], error) {
	result, err := h.svc.List(ctx, req.Msg)
	if err != nil {
		log.Printf("error listing roles: %v", err)
		return nil, err
	}
	return connect.NewResponse(&services.ListRolesResponse{
		Data:   result.GetData(),
		Cursor: result.GetCursor(),
		Count:  result.GetCount(),
	}), nil
}

func (h *RoleHandler) UpdateRole(ctx context.Context, req *connect.Request[services.UpdateRoleRequest]) (*connect.Response[services.UpdateRoleResponse], error) {
	result, err := h.svc.Update(ctx, req.Msg.GetUuid(), req.Msg.GetData())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.UpdateRoleResponse{Data: result}), nil
}

func (h *RoleHandler) DeleteRole(ctx context.Context, req *connect.Request[services.DeleteRoleRequest]) (*connect.Response[services.DeleteRoleResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetUuid()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.DeleteRoleResponse{}), nil
}
