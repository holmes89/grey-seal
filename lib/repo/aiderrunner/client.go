// Package aiderrunner implements agent.SessionRunner by running Aider
// (https://aider.chat) inside disposable Docker containers, pointed at a
// self-hosted LiteLLM Proxy (itself backed by Ollama). This replaces the
// Anthropic Managed Agents integration with a fully local, non-metered
// stack — see the aider-litellm-agent-runner plan for the full design.
package aiderrunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.uber.org/zap"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
)

const (
	defaultReapGrace    = 10 * time.Minute
	defaultReapInterval = time.Minute
)

// dockerClient is the subset of *client.Client this package uses, scoped
// down so it can be faked in tests without a real Docker daemon.
type dockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
}

var _ agentsvc.SessionRunner = (*SessionRunner)(nil)

// SessionRunner drives grey-seal agent runs via Aider running in disposable
// Docker containers. The "session ID" this type hands back to the agent
// package is the container ID.
type SessionRunner struct {
	docker         dockerClient
	image          string
	litellmBaseURL string
	litellmAPIKey  string
	litellmModel   string
	network        string
	logger         *zap.Logger

	reapGrace    time.Duration
	reapInterval time.Duration

	mu       sync.Mutex
	finished map[string]time.Time // containerID -> when GetSessionStatus/StreamSession first observed it terminal
}

// NewSessionRunner constructs a SessionRunner against the local Docker
// daemon (resolved via the standard DOCKER_HOST / socket env conventions)
// and starts its background reaper goroutine, which removes finished
// containers after a grace window (see reapLoop). networkName attaches
// spawned Aider containers to that Docker network so they can resolve
// sibling services (litellm, etc.) by name; when empty, containers land on
// the daemon's default bridge network as before.
func NewSessionRunner(image, litellmBaseURL, litellmAPIKey, litellmModel, networkName string, logger *zap.Logger) (*SessionRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return newSessionRunner(cli, image, litellmBaseURL, litellmAPIKey, litellmModel, networkName, logger), nil
}

func newSessionRunner(docker dockerClient, image, litellmBaseURL, litellmAPIKey, litellmModel, networkName string, logger *zap.Logger) *SessionRunner {
	r := &SessionRunner{
		docker:         docker,
		image:          image,
		litellmBaseURL: litellmBaseURL,
		litellmAPIKey:  litellmAPIKey,
		litellmModel:   litellmModel,
		network:        networkName,
		logger:         logger,
		reapGrace:      defaultReapGrace,
		reapInterval:   defaultReapInterval,
		finished:       make(map[string]time.Time),
	}
	go r.reapLoop(context.Background())
	return r
}

// StartSession creates and starts an Aider container for the given task.
// All the git/aider orchestration (clone, checkout, run, push) lives in the
// image's entrypoint script — this method's job is only to hand it the
// right environment and get it running; the container's exit code is the
// sole success signal GetSessionStatus needs.
func (r *SessionRunner) StartSession(ctx context.Context, req agentsvc.RunAgentTaskRequest) (string, error) {
	if req.PushBranch == "" {
		return "", fmt.Errorf("aiderrunner: PushBranch is required")
	}

	taskDescription := req.TaskDescription
	if req.Rubric != "" {
		taskDescription = fmt.Sprintf("%s\n\nAcceptance criteria:\n%s", taskDescription, req.Rubric)
	}

	env := []string{
		"REPO_URL=" + req.RepoURL,
		"PUSH_BRANCH=" + req.PushBranch,
		"GITHUB_TOKEN=" + req.GithubToken,
		"TASK_DESCRIPTION=" + taskDescription,
		"OPENAI_API_BASE=" + r.litellmBaseURL,
		"OPENAI_API_KEY=" + r.litellmAPIKey,
		"AIDER_MODEL=" + r.litellmModel,
	}
	if req.Branch != "" {
		env = append(env, "BASE_BRANCH="+req.Branch)
	}

	var networkingConfig network.NetworkingConfig
	if r.network != "" {
		networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			r.network: {},
		}
	}

	created, err := r.docker.ContainerCreate(ctx,
		&container.Config{
			Image: r.image,
			Env:   env,
			// Tty avoids Docker's stdout/stderr stdcopy framing, so
			// StreamSession can read logs as plain lines.
			Tty: true,
		},
		&container.HostConfig{
			// AutoRemove is deliberately off — the reaper goroutine owns
			// removal after a grace window, so logs remain inspectable
			// (docker logs <id>) for a while after the run finishes.
			AutoRemove: false,
		},
		&networkingConfig, nil, "",
	)
	if err != nil {
		return "", fmt.Errorf("failed to create aider container: %w", err)
	}

	if err := r.docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start aider container: %w", err)
	}

	r.logger.Info("started aider container",
		zap.String("container_id", created.ID),
		zap.String("repo_url", req.RepoURL),
		zap.String("push_branch", req.PushBranch),
	)
	return created.ID, nil
}

// GetSessionStatus maps container state onto the vocabulary
// agent.watchForCompletion expects: "running" while the container is up;
// "terminated" once it exits, with outcomeResult "satisfied" (exit 0) or
// "failed" (nonzero) — Aider/the entrypoint script are the ones deciding
// success, this just reads their verdict off the exit code.
func (r *SessionRunner) GetSessionStatus(ctx context.Context, sessionID string) (string, string, error) {
	inspect, err := r.docker.ContainerInspect(ctx, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect aider container %s: %w", sessionID, err)
	}

	if inspect.State != nil && inspect.State.Running {
		return "running", "", nil
	}

	outcome := "failed"
	if inspect.State != nil && inspect.State.ExitCode == 0 {
		outcome = "satisfied"
	}
	r.markFinished(sessionID)
	return "terminated", outcome, nil
}

// StreamSession tails the container's combined stdout/stderr (Aider's own
// progress output) line by line, then emits one final synthetic
// session.status_terminated event once the container exits — mirroring the
// terminal-signal shape the old Managed Agents adapter produced, so
// consumers of AgentRunEvent don't need to change.
func (r *SessionRunner) StreamSession(ctx context.Context, sessionID string, stream func(agentsvc.AgentRunEvent) error) error {
	logs, err := r.docker.ContainerLogs(ctx, sessionID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to open aider container logs %s: %w", sessionID, err)
	}
	defer logs.Close() //nolint:errcheck

	scanner := bufio.NewScanner(logs)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := stream(agentsvc.AgentRunEvent{Type: "agent.message", Message: line}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading aider container logs %s: %w", sessionID, err)
	}

	outcome := "failed"
	if inspect, err := r.docker.ContainerInspect(ctx, sessionID); err == nil && inspect.State != nil && inspect.State.ExitCode == 0 {
		outcome = "satisfied"
	}
	r.markFinished(sessionID)
	return stream(agentsvc.AgentRunEvent{Type: "session.status_terminated", Status: outcome})
}

func (r *SessionRunner) markFinished(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, seen := r.finished[containerID]; !seen {
		r.finished[containerID] = time.Now()
	}
}

// reapLoop periodically removes containers that finished more than
// reapGrace ago. The grace window is deliberately generous (see
// defaultReapGrace) — it's the buffer that lets a slow client finish
// polling GetSessionStatus or streaming logs after the run ends, and
// (since Docker keeps logs for stopped-but-not-removed containers) it's
// also your `docker logs <id>` debugging window before cleanup happens.
func (r *SessionRunner) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(r.reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reapOnce(ctx)
		}
	}
}

func (r *SessionRunner) reapOnce(ctx context.Context) {
	cutoff := time.Now().Add(-r.reapGrace)

	r.mu.Lock()
	var due []string
	for id, finishedAt := range r.finished {
		if finishedAt.Before(cutoff) {
			due = append(due, id)
		}
	}
	r.mu.Unlock()

	for _, id := range due {
		if err := r.docker.ContainerRemove(ctx, id, container.RemoveOptions{RemoveVolumes: true}); err != nil {
			r.logger.Warn("failed to remove finished aider container", zap.String("container_id", id), zap.Error(err))
			continue
		}
		r.logger.Info("removed finished aider container", zap.String("container_id", id))
		r.mu.Lock()
		delete(r.finished, id)
		r.mu.Unlock()
	}
}
