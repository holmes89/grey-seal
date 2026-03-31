package resource

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ ResourceService = (*resourceService)(nil)

type resourceService struct {
	resourceRepo base.Repository[*greysealv1.Resource]
	indexer      Indexer // nil-safe; indexing is skipped when nil
	logger       *zap.Logger
}

func NewResourceService(
	resourceRepo base.Repository[*greysealv1.Resource],
	indexer Indexer,
	logger *zap.Logger,
) ResourceService {
	return &resourceService{
		resourceRepo: resourceRepo,
		indexer:      indexer,
		logger:       logger,
	}
}

func (srv *resourceService) List(ctx context.Context, lis base.ListRequest) (base.ListResponse[*greysealv1.Resource], error) {
	srv.logger.Info("listing resources")
	count := uint(lis.GetCount())
	if count == 0 {
		count = 20
	}
	data, err := srv.resourceRepo.List(ctx, lis.GetCursor(), count, nil)
	if err != nil {
		srv.logger.Error("failed to list resources", zap.Error(err))
	}
	return &base.ListGenericResponse[*greysealv1.Resource]{
		Cursor: "",
		Count:  int32(len(data)),
		Data:   data,
	}, err
}

func (srv *resourceService) Get(ctx context.Context, get base.GetRequest[*greysealv1.Resource]) (base.GetResponse[*greysealv1.Resource], error) {
	srv.logger.Info("getting resource", zap.String("uuid", get.GetUuid()))
	data, err := srv.resourceRepo.Get(ctx, get.GetUuid())
	if err != nil {
		srv.logger.Error("failed to get resource", zap.String("uuid", get.GetUuid()), zap.Error(err))
	}
	return &base.GetGenericResponse[*greysealv1.Resource]{Data: data}, err
}

func (srv *resourceService) Ingest(ctx context.Context, data *greysealv1.Resource) (*greysealv1.Resource, error) {
	if data.Uuid == "" {
		data.Uuid = uuid.New().String()
	}
	now := timestamppb.New(time.Now())
	data.CreatedAt = now
	// IndexedAt remains zero until the worker confirms the content has been vectorised.
	data.IndexedAt = timestamppb.New(time.Time{})

	srv.logger.Info("ingesting resource", zap.String("name", data.GetName()), zap.String("uuid", data.GetUuid()))
	if err := srv.resourceRepo.Create(ctx, data); err != nil {
		srv.logger.Error("failed to persist resource", zap.Error(err))
		return nil, err
	}

	if srv.indexer != nil {
		if err := srv.indexer.Index(ctx, data); err != nil {
			// Indexing is best-effort: the resource is saved and can be re-indexed later.
			srv.logger.Error("failed to index resource", zap.String("uuid", data.Uuid), zap.Error(err))
		}
	}

	return data, nil
}

func (srv *resourceService) Delete(ctx context.Context, id string) error {
	srv.logger.Info("deleting resource", zap.String("uuid", id))
	err := srv.resourceRepo.Delete(ctx, id)
	if err != nil {
		srv.logger.Error("failed to delete resource", zap.String("uuid", id), zap.Error(err))
	}
	return err
}
