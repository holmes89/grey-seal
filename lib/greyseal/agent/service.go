package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ AgentService = (*agentService)(nil)

type agentService struct {
	runner SessionRunner
	repo   AgentRunRepository
	logger *zap.Logger
}

func NewAgentService(runner SessionRunner, repo AgentRunRepository, logger *zap.Logger) AgentService {
	return &agentService{runner: runner, repo: repo, logger: logger}
}

func (srv *agentService) RunAgentTask(ctx context.Context, req RunAgentTaskRequest) (*greysealv1.AgentRun, error) {
	if req.Provider != "claude" {
		return nil, fmt.Errorf("provider %q is not yet implemented", req.Provider)
	}

	srv.logger.Info("starting agent run",
		zap.String("provider", req.Provider),
		zap.String("repo_url", req.RepoURL),
	)

	sessionID, err := srv.runner.StartSession(ctx, req)
	if err != nil {
		srv.logger.Error("failed to start agent session", zap.Error(err))
		return nil, fmt.Errorf("failed to start agent session: %w", err)
	}

	now := timestamppb.New(time.Now())
	run := &greysealv1.AgentRun{
		Uuid:      uuid.New().String(),
		Provider:  req.Provider,
		RepoUrl:   req.RepoURL,
		Status:    "running",
		SessionId: sessionID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := srv.repo.Create(ctx, run); err != nil {
		srv.logger.Error("failed to persist agent run", zap.Error(err))
		return nil, fmt.Errorf("failed to persist agent run: %w", err)
	}

	srv.logger.Info("agent run started", zap.String("uuid", run.Uuid), zap.String("session_id", sessionID))
	return run, nil
}

// GetAgentRun refreshes the run's status from the live provider session
// before returning it, since the provider is the source of truth while a
// session is still active.
func (srv *agentService) GetAgentRun(ctx context.Context, runUUID string) (*greysealv1.AgentRun, error) {
	run, err := srv.repo.Get(ctx, runUUID)
	if err != nil {
		srv.logger.Error("failed to get agent run", zap.String("uuid", runUUID), zap.Error(err))
		return nil, err
	}

	if run.Status == "terminated" || run.Status == "error" || run.SessionId == "" {
		return run, nil
	}

	status, prURL, err := srv.runner.GetSessionStatus(ctx, run.SessionId)
	if err != nil {
		srv.logger.Warn("failed to refresh agent run status from provider",
			zap.String("uuid", runUUID), zap.String("session_id", run.SessionId), zap.Error(err),
		)
		return run, nil
	}
	if status == run.Status && prURL == run.PrUrl {
		return run, nil
	}

	run.Status = status
	run.PrUrl = prURL
	run.UpdatedAt = timestamppb.New(time.Now())
	if err := srv.repo.Update(ctx, runUUID, run); err != nil {
		srv.logger.Warn("failed to persist refreshed agent run status", zap.String("uuid", runUUID), zap.Error(err))
	}
	return run, nil
}

func (srv *agentService) StreamAgentRun(ctx context.Context, runUUID string, stream func(event AgentRunEvent) error) error {
	run, err := srv.repo.Get(ctx, runUUID)
	if err != nil {
		return fmt.Errorf("failed to load agent run: %w", err)
	}
	return srv.runner.StreamSession(ctx, run.SessionId, stream)
}
