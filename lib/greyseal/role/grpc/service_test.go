package grpc_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/holmes89/archaea/base"
	grpchandler "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/greyseal/role/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RoleGRPCHandlerTestSuite struct {
	suite.Suite
	svc     *mocks.MockRoleService
	handler *grpchandler.RoleHandler
}

func (s *RoleGRPCHandlerTestSuite) SetupTest() {
	s.svc = mocks.NewMockRoleService(s.T())
	s.handler = grpchandler.NewRoleHandler(s.svc)
}

func (s *RoleGRPCHandlerTestSuite) TestListRoles() {
	roles := []*v1.Role{{Uuid: "r1", Name: "Assistant"}}
	listResp := &base.ListGenericResponse[*v1.Role]{Data: roles, Count: 1}
	s.svc.On("List", mock.Anything, mock.Anything).Return(listResp, nil)

	req := connect.NewRequest(&services.ListRolesRequest{})
	resp, err := s.handler.ListRoles(context.Background(), req)
	s.Require().NoError(err)
	s.Len(resp.Msg.GetData(), 1)
}

func (s *RoleGRPCHandlerTestSuite) TestGetRole() {
	r := &v1.Role{Uuid: "r2", Name: "Writer"}
	getResp := &base.GetGenericResponse[*v1.Role]{Data: r}
	s.svc.On("Get", mock.Anything, mock.Anything).Return(getResp, nil)

	req := connect.NewRequest(&services.GetRoleRequest{Uuid: "r2"})
	resp, err := s.handler.GetRole(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("r2", resp.Msg.GetData().GetUuid())
}

func (s *RoleGRPCHandlerTestSuite) TestCreateRole() {
	created := &base.CreateGenericResponse[*v1.Role]{Data: &v1.Role{Uuid: "r3", Name: "Coder"}}
	s.svc.On("Create", mock.Anything, mock.Anything).Return(created, nil)

	req := connect.NewRequest(&services.CreateRoleRequest{Data: &v1.Role{Name: "Coder"}})
	resp, err := s.handler.CreateRole(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("r3", resp.Msg.GetData().GetUuid())
}

func (s *RoleGRPCHandlerTestSuite) TestUpdateRole() {
	updated := &v1.Role{Uuid: "r4", Name: "Updated"}
	s.svc.On("Update", mock.Anything, "r4", mock.Anything).Return(updated, nil)

	req := connect.NewRequest(&services.UpdateRoleRequest{Uuid: "r4", Data: &v1.Role{Name: "Updated"}})
	resp, err := s.handler.UpdateRole(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("Updated", resp.Msg.GetData().GetName())
}

func (s *RoleGRPCHandlerTestSuite) TestDeleteRole() {
	s.svc.On("Delete", mock.Anything, "r5").Return(nil)

	req := connect.NewRequest(&services.DeleteRoleRequest{Uuid: "r5"})
	_, err := s.handler.DeleteRole(context.Background(), req)
	s.Require().NoError(err)
}

func (s *RoleGRPCHandlerTestSuite) TestDeleteRole_Error() {
	s.svc.On("Delete", mock.Anything, "bad").Return(errors.New("not found"))

	req := connect.NewRequest(&services.DeleteRoleRequest{Uuid: "bad"})
	_, err := s.handler.DeleteRole(context.Background(), req)
	s.Require().Error(err)
}

func TestRoleGRPCHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(RoleGRPCHandlerTestSuite))
}
