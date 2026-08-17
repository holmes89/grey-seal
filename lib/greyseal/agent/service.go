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

const (
	defaultWatchPollInterval = 20 * time.Second
	defaultWatchTimeout      = time.Hour
)

// terminalOutcomeResults are the Managed Agents outcome-evaluation results
// that mean the run is done and no further polling is useful.
var terminalOutcomeResults = map[string]bool{
	"satisfied":              true,
	"failed":                 true,
	"max_iterations_reached": true,
	"interrupted":            true,
}

type agentService struct {
	runner   SessionRunner
	repo     AgentRunRepository
	prOpener PullRequestOpener
	logger   *zap.Logger

	pollInterval time.Duration
	watchTimeout time.Duration
}

func NewAgentService(runner SessionRunner, repo AgentRunRepository, prOpener PullRequestOpener, logger *zap.Logger) AgentService {
	return &agentService{
		runner:       runner,
		repo:         repo,
		prOpener:     prOpener,
		logger:       logger,
		pollInterval: defaultWatchPollInterval,
		watchTimeout: defaultWatchTimeout,
	}
}

func (srv *agentService) RunAgentTask(ctx context.Context, req RunAgentTaskRequest) (*greysealv1.AgentRun, error) {
	if req.Provider != "claude" {
		return nil, fmt.Errorf("provider %q is not yet implemented", req.Provider)
	}

	runUUID := uuid.New().String()
	branchName := "agent/" + runUUID

	srv.logger.Info("starting agent run",
		zap.String("provider", req.Provider),
		zap.String("repo_url", req.RepoURL),
		zap.String("branch", branchName),
	)

	startReq := req
	startReq.TaskDescription = withBranchInstructions(req.TaskDescription, branchName)

	sessionID, err := srv.runner.StartSession(ctx, startReq)
	if err != nil {
		srv.logger.Error("failed to start agent session", zap.Error(err))
		return nil, fmt.Errorf("failed to start agent session: %w", err)
	}

	now := timestamppb.New(time.Now())
	run := &greysealv1.AgentRun{
		Uuid:      runUUID,
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

	go srv.watchForCompletion(context.Background(), watchParams{
		runUUID:     runUUID,
		sessionID:   sessionID,
		githubToken: req.GithubToken,
		repoURL:     req.RepoURL,
		branch:      branchName,
		title:       prTitle(req.TaskDescription),
		body:        req.TaskDescription,
	})

	return run, nil
}

// withBranchInstructions appends the branch/PR instructions every agent run
// needs, regardless of provider: push to a specific, grey-seal-assigned
// branch, and don't open a pull request — grey-seal does that once the
// outcome is satisfied (see watchForCompletion), since no GitHub MCP
// integration is wired up yet.
func withBranchInstructions(taskDescription, branchName string) string {
	return fmt.Sprintf(
		"%s\n\nWhen your changes satisfy the rubric, commit them and push to a new branch named exactly %q on the \"origin\" remote. Do not open a pull request yourself — that is handled separately.",
		taskDescription, branchName,
	)
}

// prTitle derives a short PR title from a (possibly long, multi-line) task
// description — the first line, truncated.
func prTitle(taskDescription string) string {
	title := taskDescription
	for i, r := range title {
		if r == '\n' {
			title = title[:i]
			break
		}
	}
	const maxLen = 72
	const ellipsis = "…" // 3 bytes in UTF-8
	if len(title) > maxLen {
		title = title[:maxLen-len(ellipsis)] + ellipsis
	}
	if title == "" {
		title = "Automated agent run"
	}
	return title
}

type watchParams struct {
	runUUID     string
	sessionID   string
	githubToken string
	repoURL     string
	branch      string
	title       string
	body        string
}

// watchForCompletion polls the session until it reaches a terminal state,
// keeping the persisted AgentRun's status current, and opens a pull request
// the moment the outcome is satisfied. Runs detached from the originating
// request (its own background context + bounded timeout) so it isn't
// contingent on a client staying connected — but it also doesn't survive a
// process restart; see the plan doc for the accepted tradeoff.
func (srv *agentService) watchForCompletion(ctx context.Context, p watchParams) {
	ctx, cancel := context.WithTimeout(ctx, srv.watchTimeout)
	defer cancel()

	ticker := time.NewTicker(srv.pollInterval)
	defer ticker.Stop()

	for {
		status, outcomeResult, err := srv.runner.GetSessionStatus(ctx, p.sessionID)
		if err != nil {
			srv.logger.Warn("agent run watcher: failed to get session status",
				zap.String("uuid", p.runUUID), zap.Error(err))
		} else {
			srv.applyStatus(ctx, p, status, outcomeResult)
			if status == "terminated" || terminalOutcomeResults[outcomeResult] {
				return
			}
		}

		select {
		case <-ctx.Done():
			srv.logger.Warn("agent run watcher: timed out", zap.String("uuid", p.runUUID))
			return
		case <-ticker.C:
		}
	}
}

// applyStatus persists the latest status and, on a first-seen satisfied
// outcome, opens the pull request. Safe to call repeatedly — it only opens
// a PR once per run (guarded by the persisted PrUrl being empty).
func (srv *agentService) applyStatus(ctx context.Context, p watchParams, status, outcomeResult string) {
	run, err := srv.repo.Get(ctx, p.runUUID)
	if err != nil {
		srv.logger.Warn("agent run watcher: failed to load run", zap.String("uuid", p.runUUID), zap.Error(err))
		return
	}

	run.Status = status
	run.UpdatedAt = timestamppb.New(time.Now())

	if outcomeResult == "satisfied" && run.PrUrl == "" {
		prURL, err := srv.prOpener.OpenPullRequest(ctx, OpenPullRequestRequest{
			RepoURL: p.repoURL,
			Branch:  p.branch,
			Base:    "main",
			Title:   p.title,
			Body:    p.body,
			Token:   p.githubToken,
		})
		if err != nil {
			srv.logger.Error("agent run watcher: failed to open pull request",
				zap.String("uuid", p.runUUID), zap.Error(err))
			run.Error = err.Error()
		} else {
			srv.logger.Info("agent run watcher: opened pull request",
				zap.String("uuid", p.runUUID), zap.String("pr_url", prURL))
			run.PrUrl = prURL
		}
	}

	if err := srv.repo.Update(ctx, p.runUUID, run); err != nil {
		srv.logger.Warn("agent run watcher: failed to persist run", zap.String("uuid", p.runUUID), zap.Error(err))
	}
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

	if run.Status == "terminated" || run.SessionId == "" {
		return run, nil
	}

	status, outcomeResult, err := srv.runner.GetSessionStatus(ctx, run.SessionId)
	if err != nil {
		srv.logger.Warn("failed to refresh agent run status from provider",
			zap.String("uuid", runUUID), zap.String("session_id", run.SessionId), zap.Error(err),
		)
		return run, nil
	}
	if status == run.Status && (outcomeResult != "satisfied" || run.PrUrl != "") {
		return run, nil
	}

	run.Status = status
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
