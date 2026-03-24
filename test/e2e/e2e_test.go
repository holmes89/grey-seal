//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	conversationgrpc "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	rolesvc "github.com/holmes89/grey-seal/lib/greyseal/role"
	rolegrpc "github.com/holmes89/grey-seal/lib/greyseal/role/grpc"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	dockertestlib "github.com/ory/dockertest/v3"
	docker "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	greysealv1services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
)

var e2eBaseURL string

func TestMain(m *testing.M) {
	pool, err := dockertestlib.NewPool("")
	if err != nil {
		fmt.Printf("could not connect to docker: %v\n", err)
		os.Exit(1)
	}
	pool.MaxWait = 60 * time.Second

	resource, err := pool.RunWithOptions(&dockertestlib.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=greyseal_e2e",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		fmt.Printf("could not start postgres: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"postgres://postgres:postgres@localhost:%s/greyseal_e2e?sslmode=disable",
		resource.GetPort("5432/tcp"),
	)

	var store *repo.Conn
	if err := pool.Retry(func() error {
		var err error
		store, err = repo.NewDatabase(dsn)
		return err
	}); err != nil {
		fmt.Printf("could not connect to postgres: %v\n", err)
		os.Exit(1)
	}

	// Wire up real services
	roleRepository := &repo.RoleRepo{Conn: store}
	rolService := rolesvc.NewRoleService(roleRepository)

	convRepo := repo.NewConversationRepo(store)
	msgRepo := &repo.MessageRepo{Conn: store}
	convSvc := conversation.NewConversationService(convRepo, msgRepo, nil, nil, nil)

	mux := http.NewServeMux()
	rolePath, roleHandler := servicesconnect.NewRoleServiceHandler(rolegrpc.NewRoleHandler(rolService))
	mux.Handle(rolePath, roleHandler)
	convPath, convHandler := servicesconnect.NewConversationServiceHandler(conversationgrpc.NewConversationHandler(convSvc))
	mux.Handle(convPath, convHandler)

	srv := &http.Server{
		Addr:    ":19200",
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	go func() { _ = srv.ListenAndServe() }()
	time.Sleep(100 * time.Millisecond)

	e2eBaseURL = "http://localhost:19200"

	code := m.Run()

	_ = srv.Shutdown(context.Background())
	store.Close()
	_ = pool.Purge(resource)
	os.Exit(code)
}

type E2ETestSuite struct {
	suite.Suite
	roleClient servicesconnect.RoleServiceClient
	convClient servicesconnect.ConversationServiceClient
}

func (s *E2ETestSuite) SetupSuite() {
	s.roleClient = servicesconnect.NewRoleServiceClient(&http.Client{}, e2eBaseURL)
	s.convClient = servicesconnect.NewConversationServiceClient(&http.Client{}, e2eBaseURL)
}

func (s *E2ETestSuite) TestRole_CreateAndGet() {
	req := connect.NewRequest(&greysealv1services.CreateRoleRequest{
		Data: &greysealv1.Role{
			Uuid:         uuid.New().String(),
			Name:         "E2E Test Role",
			SystemPrompt: "You are a helpful assistant.",
		},
	})
	resp, err := s.roleClient.CreateRole(context.Background(), req)
	s.Require().NoError(err)
	roleUUID := resp.Msg.GetData().GetUuid()
	s.NotEmpty(roleUUID)

	getResp, err := s.roleClient.GetRole(context.Background(), connect.NewRequest(&greysealv1services.GetRoleRequest{Uuid: roleUUID}))
	s.Require().NoError(err)
	s.Equal("E2E Test Role", getResp.Msg.GetData().GetName())
}

func (s *E2ETestSuite) TestConversation_CreateListDelete() {
	// Create
	createResp, err := s.convClient.CreateConversation(context.Background(), connect.NewRequest(&greysealv1services.CreateConversationRequest{
		Title: "E2E Chat",
	}))
	s.Require().NoError(err)
	convUUID := createResp.Msg.GetData().GetUuid()
	s.NotEmpty(convUUID)

	// List
	listResp, err := s.convClient.ListConversations(context.Background(), connect.NewRequest(&greysealv1services.ListConversationsRequest{}))
	s.Require().NoError(err)
	s.GreaterOrEqual(len(listResp.Msg.GetData()), 1)

	// Delete
	_, err = s.convClient.DeleteConversation(context.Background(), connect.NewRequest(&greysealv1services.DeleteConversationRequest{Uuid: convUUID}))
	s.Require().NoError(err)
}

func TestE2ETestSuite(t *testing.T) {
	suite.Run(t, new(E2ETestSuite))
}
