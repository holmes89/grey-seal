package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	entity "github.com/holmes89/grey-seal/lib/greyseal/draft"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
)

type DraftHandler struct {
	servicesv1connect.UnimplementedDraftServiceHandler
	svc entity.DraftService
}

func NewDraftHandler(svc entity.DraftService) *DraftHandler {
	return &DraftHandler{svc: svc}
}

// DraftDocument streams a draft token by token, then one final message with
// the full body and any proposed specs.
func (h *DraftHandler) DraftDocument(ctx context.Context, req *connect.Request[services.DraftDocumentRequest], stream *connect.ServerStream[services.DraftDocumentChunk]) error {
	var kind entity.Kind
	switch req.Msg.GetKind() {
	case services.DraftKind_DRAFT_KIND_DISCOVERY:
		kind = entity.KindDiscovery
	case services.DraftKind_DRAFT_KIND_DESIGN:
		kind = entity.KindDesign
	case services.DraftKind_DRAFT_KIND_PROTO:
		kind = entity.KindProto
	default:
		return connect.NewError(connect.CodeInvalidArgument, errKind)
	}
	res, err := h.svc.Draft(ctx, entity.DraftRequest{
		Kind:         kind,
		Title:        req.Msg.GetTitle(),
		Source:       req.Msg.GetSource(),
		Current:      req.Msg.GetCurrent(),
		ProtoPackage: req.Msg.GetProtoPackage(),
	}, func(token string) error {
		return stream.Send(&services.DraftDocumentChunk{Token: token})
	})
	if err != nil {
		return err
	}
	specs := make([]*services.ProposedSpec, 0, len(res.Specs))
	for _, sp := range res.Specs {
		specs = append(specs, &services.ProposedSpec{Name: sp.Name, Operation: sp.Operation, DependsOn: sp.DependsOn})
	}
	return stream.Send(&services.DraftDocumentChunk{Done: true, Body: res.Body, Specs: specs})
}

var errKind = errors.New("kind must be DRAFT_KIND_DISCOVERY, DRAFT_KIND_DESIGN or DRAFT_KIND_PROTO")
