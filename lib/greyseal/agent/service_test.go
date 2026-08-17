package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/holmes89/grey-seal/lib/greyseal/agent"
	"github.com/holmes89/grey-seal/lib/greyseal/agent/mocks"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type AgentServiceTestSuite struct {
	suite.Suite
	runner *mocks.MockSessionRunner
	repo   *mocks.MockAgentRunRepository
	svc    agent.AgentService
}

func (s *AgentServiceTestSuite) SetupTest() {
	s.runner = mocks.NewMockSessionRunner(s.T())
	s.repo = mocks.NewMockAgentRunRepository(s.T())
	s.svc = agent.NewAgentService(s.runner, s.repo, zap.NewNop())
}

func TestRunAgentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AgentServiceTestSuite))
}

func (s *AgentServiceTestSuite) TestRunAgentTask_RejectsUnimplementedProvider() {
	_, err := s.svc.RunAgentTask(context.Background(), agent.RunAgentTaskRequest{
		Provider: "ollama:qwen2.5",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "not yet implemented")
}

func (s *AgentServiceTestSuite) TestRunAgentTask_Success() {
	req := agent.RunAgentTaskRequest{
		Provider:        "claude",
		RepoURL:         "https://github.com/holmes89/firefly",
		TaskDescription: "fill in the TODO(agent) markers",
		Rubric:          "go build ./... succeeds",
	}
	s.runner.On("StartSession", mock.Anything, req).Return("session-123", nil)
	s.repo.On("Create", mock.Anything, mock.MatchedBy(func(run *greysealv1.AgentRun) bool {
		return run.GetProvider() == "claude" &&
			run.GetRepoUrl() == req.RepoURL &&
			run.GetStatus() == "running" &&
			run.GetSessionId() == "session-123" &&
			run.GetUuid() != ""
	})).Return(nil)

	run, err := s.svc.RunAgentTask(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("running", run.GetStatus())
	s.Equal("session-123", run.GetSessionId())
}

func (s *AgentServiceTestSuite) TestRunAgentTask_StartSessionError() {
	req := agent.RunAgentTaskRequest{Provider: "claude", RepoURL: "https://github.com/holmes89/firefly"}
	s.runner.On("StartSession", mock.Anything, req).Return("", errors.New("boom"))

	_, err := s.svc.RunAgentTask(context.Background(), req)
	s.Require().Error(err)
}

func (s *AgentServiceTestSuite) TestGetAgentRun_RefreshesLiveStatus() {
	existing := &greysealv1.AgentRun{Uuid: "run-1", Status: "running", SessionId: "session-123"}
	s.repo.On("Get", mock.Anything, "run-1").Return(existing, nil)
	s.runner.On("GetSessionStatus", mock.Anything, "session-123").Return("idle", "https://github.com/holmes89/firefly/pull/1", nil)
	s.repo.On("Update", mock.Anything, "run-1", mock.MatchedBy(func(run *greysealv1.AgentRun) bool {
		return run.GetStatus() == "idle" && run.GetPrUrl() == "https://github.com/holmes89/firefly/pull/1"
	})).Return(nil)

	run, err := s.svc.GetAgentRun(context.Background(), "run-1")
	s.Require().NoError(err)
	s.Equal("idle", run.GetStatus())
	s.Equal("https://github.com/holmes89/firefly/pull/1", run.GetPrUrl())
}

func (s *AgentServiceTestSuite) TestGetAgentRun_SkipsRefreshWhenTerminated() {
	existing := &greysealv1.AgentRun{Uuid: "run-1", Status: "terminated", SessionId: "session-123"}
	s.repo.On("Get", mock.Anything, "run-1").Return(existing, nil)
	// No GetSessionStatus/Update expectation set — must not be called for a terminated run.

	run, err := s.svc.GetAgentRun(context.Background(), "run-1")
	s.Require().NoError(err)
	s.Equal("terminated", run.GetStatus())
}

func (s *AgentServiceTestSuite) TestStreamAgentRun_RelaysEvents() {
	existing := &greysealv1.AgentRun{Uuid: "run-1", SessionId: "session-123"}
	s.repo.On("Get", mock.Anything, "run-1").Return(existing, nil)
	s.runner.On("StreamSession", mock.Anything, "session-123", mock.AnythingOfType("func(agent.AgentRunEvent) error")).
		Run(func(args mock.Arguments) {
			cb := args.Get(2).(func(agent.AgentRunEvent) error)
			s.Require().NoError(cb(agent.AgentRunEvent{Type: "agent.message", Message: "hello"}))
		}).
		Return(nil)

	var got []agent.AgentRunEvent
	err := s.svc.StreamAgentRun(context.Background(), "run-1", func(event agent.AgentRunEvent) error {
		got = append(got, event)
		return nil
	})
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal("hello", got[0].Message)
}
