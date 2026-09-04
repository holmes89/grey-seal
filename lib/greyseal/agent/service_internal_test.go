package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

// Internal (package agent, not agent_test) white-box tests for
// watchForCompletion and small pure helpers. Hand-rolled fakes are used
// here instead of the generated mocks package deliberately: mocks imports
// agent (for the interface types in its method signatures), so importing
// mocks from an agent-package test file would be an import cycle.

type fakeRunner struct {
	mu               sync.Mutex
	statusSequence   []statusResult
	statusCallCount  int
	getSessionStatus func(callN int) (string, string, error)
}

type statusResult struct {
	status string
	result string
}

func (f *fakeRunner) StartSession(ctx context.Context, req RunAgentTaskRequest) (string, error) {
	return "", errors.New("not used by these tests")
}

func (f *fakeRunner) GetSessionStatus(ctx context.Context, sessionID string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.statusCallCount
	f.statusCallCount++
	if f.getSessionStatus != nil {
		return f.getSessionStatus(n)
	}
	if n < len(f.statusSequence) {
		r := f.statusSequence[n]
		return r.status, r.result, nil
	}
	last := f.statusSequence[len(f.statusSequence)-1]
	return last.status, last.result, nil
}

func (f *fakeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusCallCount
}

func (f *fakeRunner) StreamSession(ctx context.Context, sessionID string, stream func(event AgentRunEvent) error) error {
	return errors.New("not used by these tests")
}

type fakeRepo struct {
	mu   sync.Mutex
	runs map[string]*greysealv1.AgentRun
}

func newFakeRepo(run *greysealv1.AgentRun) *fakeRepo {
	return &fakeRepo{runs: map[string]*greysealv1.AgentRun{run.GetUuid(): run}}
}

func (f *fakeRepo) Create(ctx context.Context, run *greysealv1.AgentRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[run.GetUuid()] = run
	return nil
}

func (f *fakeRepo) Update(ctx context.Context, id string, run *greysealv1.AgentRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[id] = run
	return nil
}

func (f *fakeRepo) Get(ctx context.Context, id string) (*greysealv1.AgentRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return run, nil
}

func (f *fakeRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*greysealv1.AgentRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var wantStatuses []any
	if filter != nil {
		wantStatuses = filter["status"]
	}

	var runs []*greysealv1.AgentRun
	for _, run := range f.runs {
		if len(wantStatuses) > 0 {
			matched := false
			for _, s := range wantStatuses {
				if s == run.GetStatus() {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		runs = append(runs, run)
	}
	if limit > 0 && uint(len(runs)) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (f *fakeRepo) latest(id string) *greysealv1.AgentRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs[id]
}

type fakePROpener struct {
	mu       sync.Mutex
	calls    int
	returned string
	err      error
	lastReq  OpenPullRequestRequest
}

func (f *fakePROpener) OpenPullRequest(ctx context.Context, req OpenPullRequestRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastReq = req
	return f.returned, f.err
}

func (f *fakePROpener) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestService(runner *fakeRunner, repo *fakeRepo, prOpener *fakePROpener) *agentService {
	return &agentService{
		runner:       runner,
		repo:         repo,
		prOpener:     prOpener,
		logger:       zap.NewNop(),
		pollInterval: time.Millisecond,
		watchTimeout: 200 * time.Millisecond,
	}
}

func TestWatchForCompletion_SatisfiedOpensPullRequest(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"idle", "satisfied"}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1"})
	pr := &fakePROpener{returned: "https://github.com/holmes89/firefly/pull/1"}
	svc := newTestService(runner, repo, pr)

	svc.watchForCompletion(context.Background(), watchParams{
		runUUID: "run-1", sessionID: "session-123",
		githubToken: "tok", repoURL: "https://github.com/holmes89/firefly",
		branch: "agent/run-1", title: "Do the thing", body: "full description",
	})

	if got := pr.callCount(); got != 1 {
		t.Fatalf("expected OpenPullRequest called once, got %d", got)
	}
	if pr.lastReq.Branch != "agent/run-1" || pr.lastReq.Base != "main" || pr.lastReq.Token != "tok" {
		t.Fatalf("unexpected OpenPullRequest request: %+v", pr.lastReq)
	}
	run := repo.latest("run-1")
	if run.GetPrUrl() != "https://github.com/holmes89/firefly/pull/1" {
		t.Fatalf("expected PrUrl to be recorded, got %q", run.GetPrUrl())
	}
	if run.GetStatus() != "idle" {
		t.Fatalf("expected status idle, got %q", run.GetStatus())
	}
}

func TestWatchForCompletion_FailedDoesNotOpenPullRequest(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"idle", "failed"}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1"})
	pr := &fakePROpener{}
	svc := newTestService(runner, repo, pr)

	svc.watchForCompletion(context.Background(), watchParams{runUUID: "run-1", sessionID: "session-123"})

	if got := pr.callCount(); got != 0 {
		t.Fatalf("expected OpenPullRequest not called, got %d calls", got)
	}
	run := repo.latest("run-1")
	if run.GetPrUrl() != "" {
		t.Fatalf("expected no PrUrl recorded, got %q", run.GetPrUrl())
	}
	// Regression: a failed outcome used to leave Error empty, indistinguishable
	// from a success still waiting on its PR — see the aider-local-mode plan.
	if run.GetError() == "" {
		t.Fatal("expected a failed outcome to record a non-empty Error")
	}
}

func TestWatchForCompletion_FailedTwice_DoesNotOverwriteFirstError(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"idle", "failed"}, {"idle", "failed"}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1", Error: "first failure reason"})
	svc := newTestService(runner, repo, &fakePROpener{})

	svc.applyStatus(context.Background(), watchParams{runUUID: "run-1", sessionID: "session-123"}, "idle", "failed")

	if run := repo.latest("run-1"); run.GetError() != "first failure reason" {
		t.Fatalf("expected the original Error to be preserved, got %q", run.GetError())
	}
}

func TestWatchForCompletion_SatisfiedWithLocalRepo_DoesNotOpenPullRequest(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"idle", "satisfied"}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1"})
	pr := &fakePROpener{returned: "https://github.com/holmes89/firefly/pull/1"}
	svc := newTestService(runner, repo, pr)

	svc.watchForCompletion(context.Background(), watchParams{
		runUUID: "run-1", sessionID: "session-123",
		repoURL: "file:///data/beaver-agent-repos/build-1.git",
		branch:  "agent/run-1",
	})

	if got := pr.callCount(); got != 0 {
		t.Fatalf("expected OpenPullRequest not called for a file:// repo, got %d calls", got)
	}
	if run := repo.latest("run-1"); run.GetPrUrl() != "" {
		t.Fatalf("expected no PrUrl recorded for a file:// repo, got %q", run.GetPrUrl())
	}
}

func TestWatchForCompletion_AlreadyHasPullRequest_NotOpenedTwice(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"idle", "satisfied"}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1", PrUrl: "https://github.com/holmes89/firefly/pull/1"})
	pr := &fakePROpener{returned: "https://github.com/holmes89/firefly/pull/2"}
	svc := newTestService(runner, repo, pr)

	svc.watchForCompletion(context.Background(), watchParams{runUUID: "run-1", sessionID: "session-123"})

	if got := pr.callCount(); got != 0 {
		t.Fatalf("expected OpenPullRequest not called for a run that already has a PR, got %d calls", got)
	}
	if run := repo.latest("run-1"); run.GetPrUrl() != "https://github.com/holmes89/firefly/pull/1" {
		t.Fatalf("expected the original PrUrl to be preserved, got %q", run.GetPrUrl())
	}
}

func TestWatchForCompletion_StopsOnSessionTerminated(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"terminated", ""}}}
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1"})
	svc := newTestService(runner, repo, &fakePROpener{})

	svc.watchForCompletion(context.Background(), watchParams{runUUID: "run-1", sessionID: "session-123"})

	if got := runner.callCount(); got != 1 {
		t.Fatalf("expected exactly one GetSessionStatus call before stopping, got %d", got)
	}
}

func TestWatchForCompletion_StopsPollingAfterTimeout(t *testing.T) {
	runner := &fakeRunner{statusSequence: []statusResult{{"running", "running"}}} // never terminal
	repo := newFakeRepo(&greysealv1.AgentRun{Uuid: "run-1"})
	svc := newTestService(runner, repo, &fakePROpener{})

	done := make(chan struct{})
	go func() {
		svc.watchForCompletion(context.Background(), watchParams{runUUID: "run-1", sessionID: "session-123"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchForCompletion did not return within its bounded timeout")
	}
	if got := runner.callCount(); got < 2 {
		t.Fatalf("expected watcher to poll more than once before timing out, got %d calls", got)
	}
}

func TestPRTitle(t *testing.T) {
	if got := prTitle("Fix the thing\n\nmore detail"); got != "Fix the thing" {
		t.Fatalf("expected only the first line, got %q", got)
	}
	if got := prTitle(""); got != "Automated agent run" {
		t.Fatalf("expected a default title for empty input, got %q", got)
	}
	if got := prTitle(strings.Repeat("x", 200)); len(got) > 72 {
		t.Fatalf("expected long single-line titles to be truncated, got length %d", len(got))
	}
}

func TestWithBranchInstructions(t *testing.T) {
	got := withBranchInstructions("do the task", "agent/abc")
	if got == "do the task" {
		t.Fatal("expected branch instructions to be appended")
	}
	if !strings.Contains(got, "agent/abc") {
		t.Fatal("expected the branch name to appear in the augmented description")
	}
	if !strings.Contains(got, "do the task") {
		t.Fatal("expected the original task description to be preserved")
	}
}

func TestListAgentRuns_NoFilter_ReturnsEverything(t *testing.T) {
	repo := &fakeRepo{runs: map[string]*greysealv1.AgentRun{
		"run-1": {Uuid: "run-1", Status: "running"},
		"run-2": {Uuid: "run-2", Status: "terminated"},
	}}
	svc := newTestService(&fakeRunner{}, repo, &fakePROpener{})

	runs, err := svc.ListAgentRuns(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("ListAgentRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs with no filter, got %d", len(runs))
	}
}

func TestListAgentRuns_StatusFilter_OnlyMatchingReturned(t *testing.T) {
	repo := &fakeRepo{runs: map[string]*greysealv1.AgentRun{
		"run-1": {Uuid: "run-1", Status: "running"},
		"run-2": {Uuid: "run-2", Status: "terminated"},
		"run-3": {Uuid: "run-3", Status: "running"},
	}}
	svc := newTestService(&fakeRunner{}, repo, &fakePROpener{})

	runs, err := svc.ListAgentRuns(context.Background(), "running", 0)
	if err != nil {
		t.Fatalf("ListAgentRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 running runs, got %d", len(runs))
	}
	for _, r := range runs {
		if r.GetStatus() != "running" {
			t.Fatalf("expected only running runs, got one with status %q", r.GetStatus())
		}
	}
}
