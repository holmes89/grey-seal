//go:build integration

package repo_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/holmes89/archaea/testutil"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	convUUID1 = "00000000-0000-0000-0000-000000000001"
	convUUID2 = "00000000-0000-0000-0000-000000000002"
	convUUID3 = "00000000-0000-0000-0000-000000000003"
	convUUID4 = "00000000-0000-0000-0000-000000000004"
	roleUUID1 = "00000000-0000-0000-0000-000000000011"
	roleUUID2 = "00000000-0000-0000-0000-000000000012"
	roleUUID3 = "00000000-0000-0000-0000-000000000013"
)

// integrationDSN holds the full postgres:// URL used by repo.NewDatabase.
var integrationDSN string

func TestMain(m *testing.M) {
	dsn, cleanup := testutil.SetupPostgres("postgres", "postgres", "greyseal_test", func(dsn string) error {
		_, err := repo.NewDatabase("postgres://" + dsn)
		return err
	})
	integrationDSN = "postgres://" + dsn

	code := m.Run()
	cleanup()
	os.Exit(code)
}

// --- Conversation repo suite ---

type ConversationRepoTestSuite struct {
	suite.Suite
	db   *repo.Conn
	conv *repo.ConversationRepo
}

func (s *ConversationRepoTestSuite) SetupTest() {
	db, err := repo.NewDatabase(integrationDSN)
	s.Require().NoError(err)
	s.db = db
	s.conv = repo.NewConversationRepo(db)
}

func (s *ConversationRepoTestSuite) TearDownTest() {
	_, _ = s.db.DB().Exec("DELETE FROM messages")
	_, _ = s.db.DB().Exec("DELETE FROM conversations")
	s.db.Close()
}

func (s *ConversationRepoTestSuite) TestCreateAndGet() {
	c := &v1.Conversation{
		Uuid:      convUUID1,
		Title:     "Integration Test Chat",
		CreatedAt: timestamppb.New(time.Now()),
		UpdatedAt: timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.conv.Create(context.Background(), c))

	got, err := s.conv.Get(context.Background(), c.Uuid)
	s.Require().NoError(err)
	s.Equal(c.Uuid, got.GetUuid())
	s.Equal("Integration Test Chat", got.GetTitle())
}

func (s *ConversationRepoTestSuite) TestUpdate() {
	c := &v1.Conversation{
		Uuid:      convUUID2,
		Title:     "Before Update",
		CreatedAt: timestamppb.New(time.Now()),
		UpdatedAt: timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.conv.Create(context.Background(), c))

	c.Title = "After Update"
	s.Require().NoError(s.conv.Update(context.Background(), c.Uuid, c))

	got, err := s.conv.Get(context.Background(), c.Uuid)
	s.Require().NoError(err)
	s.Equal("After Update", got.GetTitle())
}

func (s *ConversationRepoTestSuite) TestDelete() {
	c := &v1.Conversation{
		Uuid:      convUUID3,
		Title:     "To Delete",
		CreatedAt: timestamppb.New(time.Now()),
		UpdatedAt: timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.conv.Create(context.Background(), c))
	s.Require().NoError(s.conv.Delete(context.Background(), c.Uuid))

	_, err := s.conv.Get(context.Background(), c.Uuid)
	s.Require().Error(err)
}

func (s *ConversationRepoTestSuite) TestList() {
	listUUIDs := [3]string{convUUID1, convUUID2, convUUID3}
	for i := 0; i < 3; i++ {
		c := &v1.Conversation{
			Uuid:      listUUIDs[i],
			Title:     fmt.Sprintf("Chat %d", i),
			CreatedAt: timestamppb.New(time.Now()),
			UpdatedAt: timestamppb.New(time.Now()),
		}
		s.Require().NoError(s.conv.Create(context.Background(), c))
	}

	list, err := s.conv.List(context.Background(), "", 10, nil)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(list), 3)
}

func TestConversationRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ConversationRepoTestSuite))
}

// --- Role repo suite ---

type RoleRepoTestSuite struct {
	suite.Suite
	db   *repo.Conn
	role *repo.RoleRepo
}

func (s *RoleRepoTestSuite) SetupTest() {
	db, err := repo.NewDatabase(integrationDSN)
	s.Require().NoError(err)
	s.db = db
	s.role = &repo.RoleRepo{Conn: db}
}

func (s *RoleRepoTestSuite) TearDownTest() {
	_, _ = s.db.DB().Exec("DELETE FROM roles")
	s.db.Close()
}

func (s *RoleRepoTestSuite) TestCreateAndGet() {
	r := &v1.Role{
		Uuid:         roleUUID1,
		Name:         "Test Role",
		SystemPrompt: "You are helpful.",
		CreatedAt:    timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.role.Create(context.Background(), r))

	got, err := s.role.Get(context.Background(), r.Uuid)
	s.Require().NoError(err)
	s.Equal("Test Role", got.GetName())
	s.Equal("You are helpful.", got.GetSystemPrompt())
}

func (s *RoleRepoTestSuite) TestUpdate() {
	r := &v1.Role{
		Uuid:         roleUUID2,
		Name:         "Before",
		SystemPrompt: "Prompt",
		CreatedAt:    timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.role.Create(context.Background(), r))

	r.Name = "After"
	s.Require().NoError(s.role.Update(context.Background(), r.Uuid, r))

	got, err := s.role.Get(context.Background(), r.Uuid)
	s.Require().NoError(err)
	s.Equal("After", got.GetName())
}

func (s *RoleRepoTestSuite) TestDelete() {
	r := &v1.Role{
		Uuid:      roleUUID3,
		Name:      "To Delete",
		CreatedAt: timestamppb.New(time.Now()),
	}
	s.Require().NoError(s.role.Create(context.Background(), r))
	s.Require().NoError(s.role.Delete(context.Background(), r.Uuid))

	_, err := s.role.Get(context.Background(), r.Uuid)
	s.Require().Error(err)
}

func TestRoleRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RoleRepoTestSuite))
}
