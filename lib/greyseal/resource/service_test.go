package resource_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/holmes89/archaea/base"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/holmes89/grey-seal/lib/greyseal/resource/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ResourceServiceTestSuite struct {
	suite.Suite
	repo    *mockResourceRepo
	indexer *mocks.MockIndexer
	svc     resource.ResourceService
}

func (s *ResourceServiceTestSuite) SetupTest() {
	s.repo = &mockResourceRepo{}
	s.indexer = mocks.NewMockIndexer(s.T())
	s.svc = resource.NewResourceService(s.repo, s.indexer, zap.NewNop())
}

func (s *ResourceServiceTestSuite) TestIngest_AssignsUUIDAndTimestamp() {
	s.indexer.On("Index", mock.Anything, mock.AnythingOfType("*greysealv1.Resource")).Return(nil)

	r, err := s.svc.Ingest(context.Background(), &v1.Resource{Name: "Test"})
	s.Require().NoError(err)
	s.NotEmpty(r.GetUuid())
	s.NotNil(r.GetCreatedAt())
	s.Equal("Test", r.GetName())
}

func (s *ResourceServiceTestSuite) TestIngest_CallsIndexer() {
	var indexed *v1.Resource
	s.indexer.On("Index", mock.Anything, mock.AnythingOfType("*greysealv1.Resource")).
		Run(func(args mock.Arguments) { indexed = args.Get(1).(*v1.Resource) }).
		Return(nil)

	r, err := s.svc.Ingest(context.Background(), &v1.Resource{Name: "Indexable"})
	s.Require().NoError(err)
	s.Equal(r.GetUuid(), indexed.GetUuid())
}

func (s *ResourceServiceTestSuite) TestIngest_IndexerErrorIsNonFatal() {
	s.indexer.On("Index", mock.Anything, mock.AnythingOfType("*greysealv1.Resource")).
		Return(errors.New("kafka unavailable"))

	_, err := s.svc.Ingest(context.Background(), &v1.Resource{Name: "Resilient"})
	// Indexer error must not propagate — resource is still created.
	s.Require().NoError(err)
}

func (s *ResourceServiceTestSuite) TestIngest_NilIndexer() {
	// Service created without an indexer should still create the resource.
	svc := resource.NewResourceService(s.repo, nil, zap.NewNop())
	_, err := svc.Ingest(context.Background(), &v1.Resource{Name: "NoIndexer"})
	s.Require().NoError(err)
}

func (s *ResourceServiceTestSuite) TestDelete() {
	s.repo.deleteErr = nil
	err := s.svc.Delete(context.Background(), "uuid-1")
	s.Require().NoError(err)
	s.Equal("uuid-1", s.repo.deletedID)
}

func TestResourceServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServiceTestSuite))
}

// ─── lightweight stub repo ────────────────────────────────────────────────────

type mockResourceRepo struct {
	created   *v1.Resource
	deletedID string
	deleteErr error
}

func (r *mockResourceRepo) Create(_ context.Context, res *v1.Resource) error {
	r.created = res
	return nil
}
func (r *mockResourceRepo) Update(_ context.Context, _ string, _ *v1.Resource) error { return nil }
func (r *mockResourceRepo) Delete(_ context.Context, id string) error {
	r.deletedID = id
	return r.deleteErr
}
func (r *mockResourceRepo) Get(_ context.Context, id string) (*v1.Resource, error) {
	return &v1.Resource{Uuid: id}, nil
}
func (r *mockResourceRepo) List(_ context.Context, _ string, _ uint, _ map[string][]any) ([]*v1.Resource, error) {
	return nil, nil
}

// Satisfy base.GetRequest[*v1.Resource] and base.ListRequest via minimal stub.
var _ base.Repository[*v1.Resource] = (*mockResourceRepo)(nil)
