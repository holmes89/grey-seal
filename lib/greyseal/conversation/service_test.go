package conversation_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

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
	// nil cache — tests that need it create their own service instance
	s.svc = conversation.NewConversationService(s.convRepo, s.msgRepo, s.searcher, s.roleRepo, s.llm, nil, zap.NewNop())
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

	// Searcher returns empty (no cache, searcher is called)
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

func (s *ConversationServiceTestSuite) TestChat_SourceAttribution() {
	convUUID := "conv-attr"
	conv := &v1.Conversation{Uuid: convUUID}

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)
	s.msgRepo.On("ListByConversation", mock.Anything, convUUID).Return([]*v1.Message{}, nil)

	// Searcher returns a titled result
	s.searcher.On("Search", mock.Anything, "query", int32(5), []string(nil)).
		Return([]conversation.SearchResult{{EntityUUID: "e1", Title: "Go Docs", Snippet: "goroutines are lightweight"}}, nil)

	// Capture the messages sent to the LLM to verify attribution format
	var capturedMessages []conversation.LLMMessage
	s.llm.On("Chat", mock.Anything, mock.MatchedBy(func(msgs []conversation.LLMMessage) bool {
		capturedMessages = msgs
		return true
	}), mock.Anything).Return("answer", nil)

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, convUUID, mock.Anything).Return(nil)

	_, err := s.svc.Chat(context.Background(), convUUID, "query", func(_ string) error { return nil })
	s.Require().NoError(err)

	// Find the system context message and verify attribution format
	var contextMsg string
	for _, m := range capturedMessages {
		if m.Role == "system" && strings.Contains(m.Content, "relevant context") {
			contextMsg = m.Content
			break
		}
	}
	s.Require().NotEmpty(contextMsg, "expected a context system message")
	s.Contains(contextMsg, "[Go Docs]: goroutines are lightweight")
}

func (s *ConversationServiceTestSuite) TestChat_SummaryPrepended() {
	convUUID := "conv-summary"
	conv := &v1.Conversation{Uuid: convUUID, Summary: "Earlier we discussed Go channels."}

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)
	s.msgRepo.On("ListByConversation", mock.Anything, convUUID).Return([]*v1.Message{}, nil)
	s.searcher.On("Search", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]conversation.SearchResult{}, nil)

	var capturedMessages []conversation.LLMMessage
	s.llm.On("Chat", mock.Anything, mock.MatchedBy(func(msgs []conversation.LLMMessage) bool {
		capturedMessages = msgs
		return true
	}), mock.Anything).Return("ok", nil)

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, convUUID, mock.Anything).Return(nil)

	_, err := s.svc.Chat(context.Background(), convUUID, "follow up", func(_ string) error { return nil })
	s.Require().NoError(err)

	// A system message containing the summary must appear before the user turn
	var hasSummary bool
	for _, m := range capturedMessages {
		if m.Role == "system" && strings.Contains(m.Content, "Earlier we discussed Go channels.") {
			hasSummary = true
			break
		}
	}
	s.True(hasSummary, "expected summary to be prepended as a system message")
}

func (s *ConversationServiceTestSuite) TestChat_CacheHit() {
	cache := mocks.NewMockResourceCache(s.T())
	svc := conversation.NewConversationService(
		s.convRepo, s.msgRepo, s.searcher, s.roleRepo, s.llm, cache, zap.NewNop(),
	)

	convUUID := "conv-cache-hit"
	conv := &v1.Conversation{Uuid: convUUID}

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)
	s.msgRepo.On("ListByConversation", mock.Anything, convUUID).Return([]*v1.Message{}, nil)

	// Cache returns a hit — searcher must NOT be called
	cache.On("List", mock.Anything, convUUID).Return([]conversation.CachedResource{
		{EntityUUID: "e1", Title: "Redis Docs", Snippet: "redis is fast", Score: 0.9},
	}, nil)

	s.llm.On("Chat", mock.Anything, mock.Anything, mock.Anything).Return("cached answer", nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, convUUID, mock.Anything).Return(nil)

	msg, err := svc.Chat(context.Background(), convUUID, "what is redis?", func(_ string) error { return nil })
	s.Require().NoError(err)
	s.Equal("cached answer", msg.GetContent())
	// searcher was NOT registered — testify mock will fail if it is called unexpectedly
}

func (s *ConversationServiceTestSuite) TestChat_CacheMiss() {
	cache := mocks.NewMockResourceCache(s.T())
	svc := conversation.NewConversationService(
		s.convRepo, s.msgRepo, s.searcher, s.roleRepo, s.llm, cache, zap.NewNop(),
	)

	convUUID := "conv-cache-miss"
	conv := &v1.Conversation{Uuid: convUUID}

	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)
	s.msgRepo.On("ListByConversation", mock.Anything, convUUID).Return([]*v1.Message{}, nil)

	// Cache miss
	cache.On("List", mock.Anything, convUUID).Return([]conversation.CachedResource(nil), nil)

	// Searcher returns results
	searchResult := []conversation.SearchResult{{EntityUUID: "e2", Title: "Kafka Docs", Snippet: "partitions scale well", Score: 0.8}}
	s.searcher.On("Search", mock.Anything, "kafka partitions", int32(5), []string(nil)).Return(searchResult, nil)

	// Cache should be populated with the results
	cache.On("Merge", mock.Anything, convUUID, mock.MatchedBy(func(rs []conversation.CachedResource) bool {
		return len(rs) == 1 && rs[0].Title == "Kafka Docs"
	})).Return(nil)

	s.llm.On("Chat", mock.Anything, mock.Anything, mock.Anything).Return("miss answer", nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, convUUID, mock.Anything).Return(nil)

	_, err := svc.Chat(context.Background(), convUUID, "kafka partitions", func(_ string) error { return nil })
	s.Require().NoError(err)
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
