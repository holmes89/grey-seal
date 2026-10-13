package grpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
)

type WorkTypeHandler struct {
	servicesv1connect.UnimplementedWorkTypeServiceHandler
	types worktype.Registry
}

func NewWorkTypeHandler(types worktype.Registry) *WorkTypeHandler {
	return &WorkTypeHandler{types: types}
}

func (h *WorkTypeHandler) ListWorkTypes(_ context.Context, _ *connect.Request[services.ListWorkTypesRequest]) (*connect.Response[services.ListWorkTypesResponse], error) {
	var data []*greysealv1.WorkType
	for _, wt := range h.types.List() {
		data = append(data, &greysealv1.WorkType{
			Id:        wt.ID,
			Label:     wt.Label,
			Prompt:    wt.Prompt,
			DraftKind: wt.DraftKind,
		})
	}
	return connect.NewResponse(&services.ListWorkTypesResponse{Data: data}), nil
}
