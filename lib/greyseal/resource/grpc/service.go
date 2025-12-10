package grpc

import (
	"context"
	"log"

	"connectrpc.com/connect"
	"github.com/holmes89/archaea/base"

	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicev1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
)

type ResourceService struct {
	servicev1.UnimplementedResourceServiceServer
	*base.GenericGRPCService[*entitiesv1.Resource]
}

func NewResourceService(svc base.Service[*entitiesv1.Resource], publisher base.Producer[*entitiesv1.Resource]) *ResourceService {
	return &ResourceService{
		GenericGRPCService: base.NewGenericGRPCService(svc, publisher),
	}
}

func (s *ResourceService) ListResources(ctx context.Context, req *connect.Request[servicev1.ListResourcesRequest]) (*connect.Response[servicev1.ListResourcesResponse], error) {
	data, err := s.List(ctx, req, req.Msg)
	if err != nil {
		log.Printf("Error from service: %v", err)
		return nil, err
	}

	log.Printf("Service returned %d resources", len(data.GetData()))
	resp := &servicev1.ListResourcesResponse{
		Data:   data.GetData(),
		Cursor: data.GetCursor(),
		Count:  data.GetCount(),
	}
	connectResp := connect.NewResponse(resp)
	return connectResp, nil
}

func (s *ResourceService) GetResource(ctx context.Context, req *connect.Request[servicev1.GetResourceRequest]) (*connect.Response[servicev1.GetResourceResponse], error) {
	data, err := s.Get(ctx, req, req.Msg)
	if err != nil {
		log.Printf("Error from service: %v", err)
		return nil, err
	}
	resp := &servicev1.GetResourceResponse{
		Data: data.GetData(),
	}
	return connect.NewResponse(resp), nil
}

func (s *ResourceService) CreateResource(ctx context.Context, req *connect.Request[servicev1.CreateResourceRequest]) (*connect.Response[servicev1.CreateResourceResponse], error) {
	id, err := s.Create(ctx, req, req.Msg)
	if err != nil {
		return nil, err
	}
	resp := req.Msg.Data
	resp.Uuid = id
	return connect.NewResponse(&servicev1.CreateResourceResponse{
		Data: resp,
	}), nil
}
