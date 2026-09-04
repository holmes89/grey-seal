package grpc

import (
	"context"

	"connectrpc.com/connect"

	entity "github.com/holmes89/grey-seal/lib/greyseal/agent"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
)

type AgentHandler struct {
	servicesconnect.UnimplementedAgentServiceHandler
	svc entity.AgentService
}

func NewAgentHandler(svc entity.AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) RunAgentTask(ctx context.Context, req *connect.Request[services.RunAgentTaskRequest]) (*connect.Response[services.RunAgentTaskResponse], error) {
	run, err := h.svc.RunAgentTask(ctx, entity.RunAgentTaskRequest{
		Provider:        req.Msg.GetProvider(),
		RepoURL:         req.Msg.GetRepoUrl(),
		GithubToken:     req.Msg.GetGithubToken(),
		Branch:          req.Msg.GetBranch(),
		TaskDescription: req.Msg.GetTaskDescription(),
		Rubric:          req.Msg.GetRubric(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.RunAgentTaskResponse{Data: run}), nil
}

func (h *AgentHandler) GetAgentRun(ctx context.Context, req *connect.Request[services.GetAgentRunRequest]) (*connect.Response[services.GetAgentRunResponse], error) {
	run, err := h.svc.GetAgentRun(ctx, req.Msg.GetUuid())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.GetAgentRunResponse{Data: run}), nil
}

func (h *AgentHandler) ListAgentRuns(ctx context.Context, req *connect.Request[services.ListAgentRunsRequest]) (*connect.Response[services.ListAgentRunsResponse], error) {
	runs, err := h.svc.ListAgentRuns(ctx, req.Msg.GetStatus(), uint(req.Msg.GetCount()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&services.ListAgentRunsResponse{
		Data:  runs,
		Count: int32(len(runs)),
	}), nil
}

// StreamAgentRun relays events for a run as they occur.
func (h *AgentHandler) StreamAgentRun(ctx context.Context, req *connect.Request[services.StreamAgentRunRequest], stream *connect.ServerStream[services.AgentRunEvent]) error {
	return h.svc.StreamAgentRun(ctx, req.Msg.GetUuid(), func(event entity.AgentRunEvent) error {
		return stream.Send(&services.AgentRunEvent{
			Type:    event.Type,
			Message: event.Message,
			Status:  event.Status,
		})
	})
}
