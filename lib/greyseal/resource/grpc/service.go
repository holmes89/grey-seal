package grpc

import (
	"context"
	"log"

	"connectrpc.com/connect"

	entity "github.com/holmes89/grey-seal/lib/greyseal/resource"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
)

type ResourceHandler struct {
	servicesconnect.UnimplementedResourceServiceHandler
	svc entity.ResourceService
}

func NewResourceHandler(svc entity.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func (h *ResourceHandler) IngestResource(ctx context.Context, req *connect.Request[services.IngestResourceRequest]) (*connect.Response[services.IngestResourceResponse], error) {
	result, err := h.svc.Ingest(ctx, req.Msg.GetData())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.IngestResourceResponse{Data: result}), nil
}

func (h *ResourceHandler) GetResource(ctx context.Context, req *connect.Request[services.GetResourceRequest]) (*connect.Response[services.GetResourceResponse], error) {
	result, err := h.svc.Get(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.GetResourceResponse{Data: result.GetData()}), nil
}

func (h *ResourceHandler) ListResources(ctx context.Context, req *connect.Request[services.ListResourcesRequest]) (*connect.Response[services.ListResourcesResponse], error) {
	result, err := h.svc.List(ctx, req.Msg)
	if err != nil {
		log.Printf("error listing resources: %v", err)
		return nil, err
	}
	return connect.NewResponse(&services.ListResourcesResponse{
		Data:   result.GetData(),
		Cursor: result.GetCursor(),
		Count:  result.GetCount(),
	}), nil
}

func (h *ResourceHandler) DeleteResource(ctx context.Context, req *connect.Request[services.DeleteResourceRequest]) (*connect.Response[services.DeleteResourceResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetUuid()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.DeleteResourceResponse{}), nil
}
