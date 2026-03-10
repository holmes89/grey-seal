package grpc

import (
	"context"
	"log"

	"connectrpc.com/connect"

	entity "github.com/holmes89/grey-seal/lib/greyseal/conversation"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
)

type ConversationHandler struct {
	servicesconnect.UnimplementedConversationServiceHandler
	svc entity.ConversationService
}

func NewConversationHandler(svc entity.ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

func (h *ConversationHandler) CreateConversation(ctx context.Context, req *connect.Request[services.CreateConversationRequest]) (*connect.Response[services.CreateConversationResponse], error) {
	conv := &greysealv1.Conversation{
		Title:         req.Msg.GetTitle(),
		RoleUuid:      req.Msg.GetRoleUuid(),
		ResourceUuids: req.Msg.GetResourceUuids(),
	}
	result, err := h.svc.Create(ctx, conv)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.CreateConversationResponse{Data: result}), nil
}

func (h *ConversationHandler) GetConversation(ctx context.Context, req *connect.Request[services.GetConversationRequest]) (*connect.Response[services.GetConversationResponse], error) {
	result, err := h.svc.Get(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.GetConversationResponse{Data: result.GetData()}), nil
}

func (h *ConversationHandler) ListConversations(ctx context.Context, req *connect.Request[services.ListConversationsRequest]) (*connect.Response[services.ListConversationsResponse], error) {
	result, err := h.svc.List(ctx, req.Msg)
	if err != nil {
		log.Printf("error listing conversations: %v", err)
		return nil, err
	}
	return connect.NewResponse(&services.ListConversationsResponse{
		Data:   result.GetData(),
		Cursor: result.GetCursor(),
		Count:  result.GetCount(),
	}), nil
}

func (h *ConversationHandler) UpdateConversation(ctx context.Context, req *connect.Request[services.UpdateConversationRequest]) (*connect.Response[services.UpdateConversationResponse], error) {
	conv := &greysealv1.Conversation{
		Uuid:          req.Msg.GetUuid(),
		Title:         req.Msg.GetTitle(),
		RoleUuid:      req.Msg.GetRoleUuid(),
		ResourceUuids: req.Msg.GetResourceUuids(),
	}
	result, err := h.svc.Update(ctx, req.Msg.GetUuid(), conv)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.UpdateConversationResponse{Data: result}), nil
}

func (h *ConversationHandler) DeleteConversation(ctx context.Context, req *connect.Request[services.DeleteConversationRequest]) (*connect.Response[services.DeleteConversationResponse], error) {
	if err := h.svc.Delete(ctx, req.Msg.GetUuid()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.DeleteConversationResponse{}), nil
}

// Chat streams assistant tokens back to the client as they are generated.
func (h *ConversationHandler) Chat(ctx context.Context, req *connect.Request[services.ChatRequest], stream *connect.ServerStream[services.ChatResponse]) error {
	finalMsg, err := h.svc.Chat(ctx, req.Msg.GetConversationUuid(), req.Msg.GetContent(),
		func(token string) error {
			return stream.Send(&services.ChatResponse{Token: token})
		},
	)
	if err != nil {
		return err
	}
	// Send a final message with the fully-populated Message (uuid, references, etc.)
	return stream.Send(&services.ChatResponse{FinalMessage: finalMsg})
}

func (h *ConversationHandler) SubmitFeedback(ctx context.Context, req *connect.Request[services.SubmitFeedbackRequest]) (*connect.Response[services.SubmitFeedbackResponse], error) {
	if err := h.svc.SubmitFeedback(ctx, req.Msg.GetMessageUuid(), req.Msg.GetFeedback()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.SubmitFeedbackResponse{}), nil
}
