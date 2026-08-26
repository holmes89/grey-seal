package aiderrunner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
)

// fakeDockerClient is a hand-rolled fake (not a mockery mock — this
// interface is small and unexported, so a fake is simpler than wiring up
// generation for it) implementing the dockerClient interface used by
// SessionRunner.
type fakeDockerClient struct {
	createFn  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	startFn   func(ctx context.Context, containerID string, options container.StartOptions) error
	inspectFn func(ctx context.Context, containerID string) (container.InspectResponse, error)
	logsFn    func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	removeFn  func(ctx context.Context, containerID string, options container.RemoveOptions) error

	lastCreateConfig     *container.Config
	lastNetworkingConfig *network.NetworkingConfig
	removedIDs           []string
}

func (f *fakeDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	f.lastCreateConfig = config
	f.lastNetworkingConfig = networkingConfig
	if f.createFn != nil {
		return f.createFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "container-1"}, nil
}

func (f *fakeDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if f.startFn != nil {
		return f.startFn(ctx, containerID, options)
	}
	return nil
}

func (f *fakeDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if f.inspectFn != nil {
		return f.inspectFn(ctx, containerID)
	}
	return container.InspectResponse{}, nil
}

func (f *fakeDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	if f.logsFn != nil {
		return f.logsFn(ctx, containerID, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if f.removeFn != nil {
		if err := f.removeFn(ctx, containerID, options); err != nil {
			return err
		}
	}
	f.removedIDs = append(f.removedIDs, containerID)
	return nil
}

func newTestRunner(fake *fakeDockerClient) *SessionRunner {
	// Not using newSessionRunner directly here so the reaper goroutine
	// isn't started against a fake with no synchronization story for
	// concurrent test access — reap behavior is exercised directly via
	// reapOnce in its own test instead.
	return &SessionRunner{
		docker:         fake,
		image:          "aider-runner:test",
		litellmBaseURL: "http://litellm:4000",
		litellmAPIKey:  "sk-test",
		litellmModel:   "qwen-coder",
		network:        "",
		logger:         zap.NewNop(),
		reapGrace:      defaultReapGrace,
		reapInterval:   defaultReapInterval,
		finished:       make(map[string]time.Time),
	}
}

func TestStartSession_RequiresPushBranch(t *testing.T) {
	r := newTestRunner(&fakeDockerClient{})
	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{})
	assert.Error(t, err)
}

func TestStartSession_BuildsEnvAndStarts(t *testing.T) {
	fake := &fakeDockerClient{}
	r := newTestRunner(fake)

	id, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		RepoURL:         "https://github.com/owner/repo",
		Branch:          "develop",
		PushBranch:      "agent/run-1",
		GithubToken:     "ghp_token",
		TaskDescription: "do the thing",
		Rubric:          "must compile",
	})
	require.NoError(t, err)
	assert.Equal(t, "container-1", id)

	require.NotNil(t, fake.lastCreateConfig)
	env := fake.lastCreateConfig.Env
	assert.Contains(t, env, "REPO_URL=https://github.com/owner/repo")
	assert.Contains(t, env, "PUSH_BRANCH=agent/run-1")
	assert.Contains(t, env, "BASE_BRANCH=develop")
	assert.Contains(t, env, "GITHUB_TOKEN=ghp_token")
	assert.Contains(t, env, "OPENAI_API_BASE=http://litellm:4000")
	assert.Contains(t, env, "OPENAI_API_KEY=sk-test")
	assert.Contains(t, env, "AIDER_MODEL=qwen-coder")
	found := false
	for _, e := range env {
		if e == "TASK_DESCRIPTION=do the thing\n\nAcceptance criteria:\nmust compile" {
			found = true
		}
	}
	assert.True(t, found, "expected TASK_DESCRIPTION to include the rubric, got env: %v", env)
}

func TestStartSession_AttachesConfiguredNetwork(t *testing.T) {
	fake := &fakeDockerClient{}
	r := newTestRunner(fake)
	r.network = "joelholmeshaus_web"

	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		RepoURL:    "https://github.com/owner/repo",
		PushBranch: "agent/run-1",
	})
	require.NoError(t, err)

	require.NotNil(t, fake.lastNetworkingConfig)
	require.Contains(t, fake.lastNetworkingConfig.EndpointsConfig, "joelholmeshaus_web")
}

func TestStartSession_NoNetworkConfiguredLeavesEndpointsEmpty(t *testing.T) {
	fake := &fakeDockerClient{}
	r := newTestRunner(fake) // network is "" by default

	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		RepoURL:    "https://github.com/owner/repo",
		PushBranch: "agent/run-1",
	})
	require.NoError(t, err)

	require.NotNil(t, fake.lastNetworkingConfig)
	assert.Empty(t, fake.lastNetworkingConfig.EndpointsConfig)
}

func TestStartSession_OmitsBaseBranchWhenUnset(t *testing.T) {
	fake := &fakeDockerClient{}
	r := newTestRunner(fake)

	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{
		RepoURL:    "https://github.com/owner/repo",
		PushBranch: "agent/run-1",
	})
	require.NoError(t, err)

	for _, e := range fake.lastCreateConfig.Env {
		assert.NotContains(t, e, "BASE_BRANCH=")
	}
}

func TestStartSession_PropagatesCreateError(t *testing.T) {
	fake := &fakeDockerClient{
		createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{}, errors.New("boom")
		},
	}
	r := newTestRunner(fake)
	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{RepoURL: "x", PushBranch: "y"})
	assert.Error(t, err)
}

func TestStartSession_PropagatesStartError(t *testing.T) {
	fake := &fakeDockerClient{
		startFn: func(ctx context.Context, containerID string, options container.StartOptions) error {
			return errors.New("boom")
		},
	}
	r := newTestRunner(fake)
	_, err := r.StartSession(context.Background(), agentsvc.RunAgentTaskRequest{RepoURL: "x", PushBranch: "y"})
	assert.Error(t, err)
}

func TestGetSessionStatus_Running(t *testing.T) {
	fake := &fakeDockerClient{
		inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: true},
			}}, nil
		},
	}
	r := newTestRunner(fake)
	status, outcome, err := r.GetSessionStatus(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "running", status)
	assert.Equal(t, "", outcome)
	r.mu.Lock()
	_, marked := r.finished["c1"]
	r.mu.Unlock()
	assert.False(t, marked)
}

func TestGetSessionStatus_TerminatedSatisfied(t *testing.T) {
	fake := &fakeDockerClient{
		inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: false, ExitCode: 0},
			}}, nil
		},
	}
	r := newTestRunner(fake)
	status, outcome, err := r.GetSessionStatus(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "terminated", status)
	assert.Equal(t, "satisfied", outcome)
	r.mu.Lock()
	_, marked := r.finished["c1"]
	r.mu.Unlock()
	assert.True(t, marked)
}

func TestGetSessionStatus_TerminatedFailed(t *testing.T) {
	fake := &fakeDockerClient{
		inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{Running: false, ExitCode: 1},
			}}, nil
		},
	}
	r := newTestRunner(fake)
	status, outcome, err := r.GetSessionStatus(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "terminated", status)
	assert.Equal(t, "failed", outcome)
}

func TestGetSessionStatus_InspectError(t *testing.T) {
	fake := &fakeDockerClient{
		inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{}, errors.New("boom")
		},
	}
	r := newTestRunner(fake)
	_, _, err := r.GetSessionStatus(context.Background(), "c1")
	assert.Error(t, err)
}

func TestStreamSession_LinesThenTerminalEvent(t *testing.T) {
	fake := &fakeDockerClient{
		logsFn: func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
			assert.True(t, options.Follow)
			return io.NopCloser(bytes.NewBufferString("line one\nline two\n")), nil
		},
		inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
				State: &container.State{ExitCode: 0},
			}}, nil
		},
	}
	r := newTestRunner(fake)

	var events []agentsvc.AgentRunEvent
	err := r.StreamSession(context.Background(), "c1", func(e agentsvc.AgentRunEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "agent.message", events[0].Type)
	assert.Equal(t, "line one", events[0].Message)
	assert.Equal(t, "agent.message", events[1].Type)
	assert.Equal(t, "line two", events[1].Message)
	assert.Equal(t, "session.status_terminated", events[2].Type)
	assert.Equal(t, "satisfied", events[2].Status)
}

func TestStreamSession_CallbackErrorAborts(t *testing.T) {
	fake := &fakeDockerClient{
		logsFn: func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBufferString("line one\nline two\n")), nil
		},
	}
	r := newTestRunner(fake)

	wantErr := errors.New("client hung up")
	err := r.StreamSession(context.Background(), "c1", func(e agentsvc.AgentRunEvent) error {
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestReapOnce_RemovesOnlyPastGraceWindow(t *testing.T) {
	fake := &fakeDockerClient{}
	r := newTestRunner(fake)
	r.reapGrace = time.Minute

	r.finished["old"] = time.Now().Add(-2 * time.Minute)
	r.finished["recent"] = time.Now()

	r.reapOnce(context.Background())

	assert.Equal(t, []string{"old"}, fake.removedIDs)
	r.mu.Lock()
	_, oldStillTracked := r.finished["old"]
	_, recentStillTracked := r.finished["recent"]
	r.mu.Unlock()
	assert.False(t, oldStillTracked, "removed container should be dropped from tracking")
	assert.True(t, recentStillTracked, "container still within grace window should stay tracked")
}

func TestReapOnce_LogsAndContinuesOnRemoveError(t *testing.T) {
	fake := &fakeDockerClient{
		removeFn: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
			return errors.New("boom")
		},
	}
	r := newTestRunner(fake)
	r.reapGrace = time.Minute
	r.finished["old"] = time.Now().Add(-2 * time.Minute)

	r.reapOnce(context.Background())

	r.mu.Lock()
	_, stillTracked := r.finished["old"]
	r.mu.Unlock()
	assert.True(t, stillTracked, "a failed removal should stay tracked for retry on the next tick")
}
