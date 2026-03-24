package role_test

import (
	"context"
	"testing"

	"github.com/holmes89/archaea/base"
	"github.com/holmes89/grey-seal/lib/greyseal/role"
	"github.com/holmes89/grey-seal/lib/greyseal/role/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RoleServiceTestSuite struct {
	suite.Suite
	repo *mocks.MockRoleRepository
	svc  role.RoleService
}

func (s *RoleServiceTestSuite) SetupTest() {
	s.repo = mocks.NewMockRoleRepository(s.T())
	s.svc = role.NewRoleService(s.repo)
}

func (s *RoleServiceTestSuite) TestList() {
	roles := []*v1.Role{{Uuid: "r1", Name: "Assistant"}}
	s.repo.On("List", mock.Anything, "", uint(10), mock.Anything).Return(roles, nil)

	resp, err := s.svc.List(context.Background(), &fakeListReq{count: 10})
	s.Require().NoError(err)
	s.Len(resp.GetData(), 1)
	s.Equal("r1", resp.GetData()[0].GetUuid())
}

func (s *RoleServiceTestSuite) TestGet() {
	r := &v1.Role{Uuid: "r2", Name: "Writer"}
	s.repo.On("Get", mock.Anything, "r2").Return(r, nil)

	resp, err := s.svc.Get(context.Background(), &fakeGetRoleReq{uuid: "r2"})
	s.Require().NoError(err)
	s.Equal("r2", resp.GetData().GetUuid())
}

func (s *RoleServiceTestSuite) TestCreate() {
	r := &v1.Role{Uuid: "r3", Name: "Coder"}
	s.repo.On("Create", mock.Anything, r).Return(nil)

	resp, err := s.svc.Create(context.Background(), &fakeCreateRoleReq{data: r})
	s.Require().NoError(err)
	s.Equal("r3", resp.GetData().GetUuid())
}

func (s *RoleServiceTestSuite) TestUpdate() {
	r := &v1.Role{Uuid: "r4", Name: "Updated"}
	s.repo.On("Update", mock.Anything, "r4", r).Return(nil)

	result, err := s.svc.Update(context.Background(), "r4", r)
	s.Require().NoError(err)
	s.Equal("Updated", result.GetName())
}

func (s *RoleServiceTestSuite) TestDelete() {
	s.repo.On("Delete", mock.Anything, "r5").Return(nil)

	err := s.svc.Delete(context.Background(), "r5")
	s.Require().NoError(err)
}

func TestRoleServiceTestSuite(t *testing.T) {
	suite.Run(t, new(RoleServiceTestSuite))
}

type fakeListReq struct{ count int32 }

func (r *fakeListReq) GetCursor() string { return "" }
func (r *fakeListReq) GetCount() int32   { return r.count }

type fakeGetRoleReq struct{ uuid string }

func (r *fakeGetRoleReq) GetUuid() string { return r.uuid }

type fakeCreateRoleReq struct{ data *v1.Role }

func (r *fakeCreateRoleReq) GetData() *v1.Role { return r.data }
func (r *fakeCreateRoleReq) GetUuid() string   { return r.data.GetUuid() }

// Ensure fakeCreateRoleReq satisfies base.CreateRequest[*v1.Role]
var _ base.CreateRequest[*v1.Role] = (*fakeCreateRoleReq)(nil)
