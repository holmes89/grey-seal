package conversation_test

import (
	"context"
	"testing"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConversationServiceTestSuite struct {
	suite.Suite
	convRepo *mocks.MockConversationRepository
	msgRepo  *mocks.MockMessageRepository
	searcher *mocks.MockSearcher
	roleRepo *mocks.MockRoleRepository
	llm      *mocks.MockLLM
	svc      conversation.ConversationService
}

func (s *ConversationServiceTestSuite) SetupTest() {
	s.convRepo = mocks.NewMockConversationRepository(s.T())
	s.msgRepo = mocks.NewMockMessageRepository(s.T())
	s.searcher = mocks.NewMockSearcher(s.T())
	s.roleRepo = mocks.NewMockRoleRepository(s.T())
	s.llm = mocks.NewMockLLM(s.T())
	s.svc = conversation.NewConversationService(s.convRepo, s.msgRepo, s.searcher, s.roleRepo, s.llm)
}

func (s *ConversationServiceTestSuite) TestList() {
	convs := []*v1.Conversation{{Uuid: "c1", Title: "Test"}}
	s.convRepo.On("List", mock.Anything, "", uint(10), mock.Anything).Return(convs, nil)

	resp, err := s.svc.List(context.Background(), &fakeListReq{cursor: "", count: 10})
	s.Require().NoError(err)
	s.Len(resp.GetData(), 1)
	s.Equal("c1", resp.GetData()[0].GetUuid())
}

func (s *ConversationServiceTestSuite) TestGet() {
	conv := &v1.Conversation{Uuid: "abc", Title: "Chat 1"}
	s.convRepo.On("Get", mock.Anything, "abc").Return(conv, nil)

	resp, err := s.svc.Get(context.Background(), &fakeGetConvReq{uuid: "abc"})
	s.Require().NoError(err)
	s.Equal("abc", resp.GetData().GetUuid())
}

func (s *ConversationServiceTestSuite) TestCreate_AssignsUUID() {
	s.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.Title == "New Chat" && c.Uuid != ""
	})).Return(nil)

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Title: "New Chat"})
	s.Require().NoError(err)
	s.NotEmpty(result.GetUuid())
	s.Equal("New Chat", result.GetTitle())
}

func (s *ConversationServiceTestSuite) TestCreate_PreservesUUID() {
	s.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.Uuid == "existing-uuid"
	})).Return(nil)

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Uuid: "existing-uuid", Title: "Chat"})
	s.Require().NoError(err)
	s.Equal("existing-uuid", result.GetUuid())
}

func (s *ConversationServiceTestSuite) TestUpdate() {
	conv := &v1.Conversation{Uuid: "u1", Title: "Updated"}
	s.convRepo.On("Update", mock.Anything, "u1", mock.AnythingOfType("*greysealv1.Conversation")).Return(nil)

	result, err := s.svc.Update(context.Background(), "u1", conv)
	s.Require().NoError(err)
	s.Equal("Updated", result.GetTitle())
}

func (s *ConversationServiceTestSuite) TestDelete() {
	s.convRepo.On("Delete", mock.Anything, "del-1").Return(nil)

	err := s.svc.Delete(context.Background(), "del-1")
	s.Require().NoError(err)
}

func (s *ConversationServiceTestSuite) TestChat_WithLLM() {
	convUUID := "conv-1"
	conv := &v1.Conversation{Uuid: convUUID, Title: "Chat"}

	// Save user message
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER && m.Content == "hello"
	})).Return(nil).Once()

	// Load conversation
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)

	// List history (empty)
	s.msgRepo.On("ListByConversation", mock.Anything, convUUID).Return([]*v1.Message{}, nil)

	// Searcher returns empty
	s.searcher.On("Search", mock.Anything, "hello", int32(5), []string(nil)).Return([]conversation.SearchResult{}, nil)

	// LLM call
	s.llm.On("Chat", mock.Anything, mock.Anything, mock.Anything).Return("world", nil)

	// Save assistant message
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT && m.Content == "world"
	})).Return(nil).Once()

	// Update conversation timestamp
	s.convRepo.On("Update", mock.Anything, convUUID, mock.Anything).Return(nil)

	msg, err := s.svc.Chat(context.Background(), convUUID, "hello", func(_ string) error { return nil })
	s.Require().NoError(err)
	s.Equal(v1.MessageRole_MESSAGE_ROLE_ASSISTANT, msg.GetRole())
	s.Equal("world", msg.GetContent())
}

func (s *ConversationServiceTestSuite) TestSubmitFeedback() {
	s.msgRepo.On("UpdateFeedback", mock.Anything, "msg-1", int32(1)).Return(nil)

	err := s.svc.SubmitFeedback(context.Background(), "msg-1", 1)
	s.Require().NoError(err)
}

func TestConversationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConversationServiceTestSuite))
}

// fakeListReq implements base.ListRequest.
type fakeListReq struct {
	cursor string
	count  int32
}

func (r *fakeListReq) GetCursor() string { return r.cursor }
func (r *fakeListReq) GetCount() int32   { return r.count }

// fakeGetConvReq implements base.GetRequest[*v1.Conversation].
type fakeGetConvReq struct{ uuid string }

func (r *fakeGetConvReq) GetUuid() string { return r.uuid }
