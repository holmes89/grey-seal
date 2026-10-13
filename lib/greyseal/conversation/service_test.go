package conversation_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation/mocks"
	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	draftmocks "github.com/holmes89/grey-seal/lib/greyseal/draft/mocks"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConversationServiceTestSuite struct {
	suite.Suite
	convRepo *mocks.MockConversationRepository
	msgRepo  *mocks.MockMessageRepository
	searcher *mocks.MockSearcher
	types    worktype.Registry
	drafter  *draftmocks.MockDraftService
	llm      *mocks.MockLLM
	svc      conversation.ConversationService
}

func (s *ConversationServiceTestSuite) SetupTest() {
	s.convRepo = mocks.NewMockConversationRepository(s.T())
	s.msgRepo = mocks.NewMockMessageRepository(s.T())
	s.searcher = mocks.NewMockSearcher(s.T())
	types, err := worktype.Load("")
	s.Require().NoError(err)
	s.types = types
	s.drafter = draftmocks.NewMockDraftService(s.T())
	s.llm = mocks.NewMockLLM(s.T())
	// nil cache — tests that need it create their own service instance
	s.svc = conversation.NewConversationService(s.convRepo, s.msgRepo, s.searcher, s.types, s.drafter, nil, s.llm, nil, zap.NewNop(), nil)
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
	s.msgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Title: "New Chat"})
	s.Require().NoError(err)
	s.NotEmpty(result.GetUuid())
	s.Equal("New Chat", result.GetTitle())
}

func (s *ConversationServiceTestSuite) TestCreate_PreservesUUID() {
	s.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.Uuid == "existing-uuid"
	})).Return(nil)
	s.msgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Uuid: "existing-uuid", Title: "Chat"})
	s.Require().NoError(err)
	s.Equal("existing-uuid", result.GetUuid())
}

func (s *ConversationServiceTestSuite) TestCreate_StartsWithWelcome() {
	s.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.WorkType == worktype.Pending
	})).Return(nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT &&
			m.Content == "Hi! How can I help? Discovery, Design, Tickets, Research, or Something else."
	})).Return(nil).Once()

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Title: "New Chat"})
	s.Require().NoError(err)
	s.Equal(worktype.Pending, result.GetWorkType())
	s.Require().Len(result.GetMessages(), 1)
}

func (s *ConversationServiceTestSuite) TestCreate_KeepsGivenWorkTypeWithoutWelcome() {
	s.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.WorkType == "research"
	})).Return(nil)

	result, err := s.svc.Create(context.Background(), &v1.Conversation{Title: "Chat", WorkType: "research"})
	s.Require().NoError(err)
	s.Empty(result.GetMessages())
}

func (s *ConversationServiceTestSuite) chatWhilePending(content string) (*v1.Message, string) {
	convUUID := "conv-pending"
	conv := &v1.Conversation{Uuid: convUUID, Title: "Chat", WorkType: worktype.Pending}
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Get", mock.Anything, convUUID).Return(conv, nil)

	var streamed string
	msg, err := s.svc.Chat(context.Background(), convUUID, content, func(tok string) error {
		streamed += tok
		return nil
	})
	s.Require().NoError(err)
	return msg, streamed
}

func (s *ConversationServiceTestSuite) TestChat_PendingMatchSetsWorkTypeWithoutLLM() {
	s.convRepo.On("Update", mock.Anything, "conv-pending", mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.WorkType == "design"
	})).Return(nil).Once()
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT && m.Content == "What would you like to design?"
	})).Return(nil).Once()

	msg, streamed := s.chatWhilePending("  Design ")
	s.Equal("What would you like to design?", msg.GetContent())
	s.Equal(msg.GetContent(), streamed)
}

func (s *ConversationServiceTestSuite) TestChat_AcceptsChainOfferAndDraftsFromSeed() {
	seed := "# Feature Discovery: Notes\n\n## Overview\nSomething."
	conv := &v1.Conversation{
		Uuid: "conv-chain", Title: "", WorkType: "discovery",
		PendingChainOffer: "design", PendingChainOfferSource: seed,
	}
	s.convRepo.On("Get", mock.Anything, "conv-chain").Return(conv, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, "conv-chain", mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.PendingChainOffer == "" && c.WorkType == "discovery"
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, "conv-chain", mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.PendingChainOffer == "" && c.WorkType == "design"
	})).Return(nil).Once()
	s.drafter.On("Draft", mock.Anything, mock.MatchedBy(func(r draft.DraftRequest) bool {
		return r.Kind == draft.KindDesign && r.Source == seed && r.Current == ""
	}), mock.Anything).Return(&draft.Result{Body: "# Design: Notes"}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "design" && m.Content == "# Design: Notes"
	})).Return(nil).Once()

	msg, err := s.svc.Chat(context.Background(), "conv-chain", "yes", func(string) error { return nil })
	s.Require().NoError(err)
	s.Equal("design", msg.GetDraftKind())
	s.Equal("# Design: Notes", msg.GetContent())
}

func (s *ConversationServiceTestSuite) TestChat_DecliningChainOfferFallsThroughToNormalDraftHandling() {
	conv := &v1.Conversation{
		Uuid: "conv-chain2", Title: "", WorkType: "design",
		PendingChainOffer: "tickets", PendingChainOfferSource: "# Design: X",
	}
	s.convRepo.On("Get", mock.Anything, "conv-chain2").Return(conv, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, "conv-chain2", mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.PendingChainOffer == "" && c.WorkType == "design"
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-chain2").Return([]*v1.Message{
		{Uuid: "p", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "What would you like to design?"},
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "A tracker"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.Anything, mock.Anything).Return(&draft.Result{Body: "# Design: Tracker v2"}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "design" && m.Content == "# Design: Tracker v2"
	})).Return(nil).Once()

	msg, err := s.svc.Chat(context.Background(), "conv-chain2", "no thanks, not yet", func(string) error { return nil })
	s.Require().NoError(err)
	s.Equal("design", msg.GetDraftKind())
}

func (s *ConversationServiceTestSuite) TestChat_PendingMissReoffersOptions() {
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT &&
			strings.HasPrefix(m.Content, "I didn't catch that.") && strings.Contains(m.Content, "Design")
	})).Return(nil).Once()

	msg, _ := s.chatWhilePending("banana")
	s.Contains(msg.GetContent(), "Something else")
	s.convRepo.AssertNotCalled(s.T(), "Update", mock.Anything, mock.Anything, mock.Anything)
}

// --- drafting work types ---

func (s *ConversationServiceTestSuite) draftingConv(workType string) *v1.Conversation {
	return &v1.Conversation{Uuid: "conv-d", Title: "", WorkType: workType}
}

func (s *ConversationServiceTestSuite) TestChat_DraftTypeDraftsFromEverythingSinceThePrompt() {
	conv := s.draftingConv("design")
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(conv, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "w", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "Hi! How can I help?"},
		{Uuid: "u0", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "design"},
		{Uuid: "p", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "What would you like to design?"},
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "A shipping tracker"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.MatchedBy(func(r draft.DraftRequest) bool {
		return r.Kind == draft.KindDesign && r.Source == "A shipping tracker" && r.Current == "" && r.Title == "A shipping tracker"
	}), mock.Anything).Return(&draft.Result{Body: "# Design: Tracker"}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT && m.DraftKind == "design" && m.Content == "# Design: Tracker"
	})).Return(nil).Once()

	msg, err := s.svc.Chat(context.Background(), "conv-d", "A shipping tracker", func(string) error { return nil })
	s.Require().NoError(err)
	s.Equal("design", msg.GetDraftKind())
}

func (s *ConversationServiceTestSuite) TestChat_DraftWithOpenQuestionsSendsAFollowUpNudge() {
	conv := s.draftingConv("design")
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(conv, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "p", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "What would you like to design?"},
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "A shipping tracker"},
	}, nil)
	draftBody := "# Design: Tracker\n\n## Open questions\n" +
		"| # | Question | Leaning |\n| --- | --- | --- |\n" +
		"| 1 | Who owns shipment status updates? | Neutral |\n"
	s.drafter.On("Draft", mock.Anything, mock.Anything, mock.Anything).Return(&draft.Result{Body: draftBody}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "design" && m.Content == draftBody
	})).Return(nil).Once()
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "" && m.Role == v1.MessageRole_MESSAGE_ROLE_ASSISTANT &&
			strings.Contains(m.Content, "Who owns shipment status updates?") &&
			strings.Contains(m.Content, "Reply with answers to refine the draft")
	})).Return(nil).Once()

	msg, err := s.svc.Chat(context.Background(), "conv-d", "A shipping tracker", func(string) error { return nil })
	s.Require().NoError(err)
	s.Equal("design", msg.GetDraftKind())
}

func (s *ConversationServiceTestSuite) TestChat_DraftWithoutOpenQuestionsSendsNoNudge() {
	conv := s.draftingConv("tickets")
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(conv, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "Build the API"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.Anything, mock.Anything).Return(&draft.Result{Tickets: []draft.Ticket{
		{Title: "Add endpoint", Body: "Do it", AcceptanceCriteria: []string{"returns 201"}},
	}}, nil)
	// Only one Create call is expected for the assistant side (the draft
	// itself) — if a nudge were sent too, this mock would see a second,
	// unmatched Create call and the test would fail via mockery's strict
	// unexpected-call panic.
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "tickets"
	})).Return(nil).Once()

	_, err := s.svc.Chat(context.Background(), "conv-d", "Build the API", func(string) error { return nil })
	s.Require().NoError(err)
}

func (s *ConversationServiceTestSuite) TestChat_DraftTypeFollowUpRefinesLatestDraft() {
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(s.draftingConv("discovery"), nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "p", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "What would you like to explore?"},
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "Notes feature"},
		{Uuid: "d1", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "# Feature Discovery: v1", DraftKind: "discovery"},
		{Uuid: "u2", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "Add a scratch pad"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.MatchedBy(func(r draft.DraftRequest) bool {
		return r.Source == "Notes feature\n\nAdd a scratch pad" && r.Current == "# Feature Discovery: v1"
	}), mock.Anything).Return(&draft.Result{Body: "# Feature Discovery: v2"}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.DraftKind == "discovery" && m.Content == "# Feature Discovery: v2"
	})).Return(nil).Once()

	_, err := s.svc.Chat(context.Background(), "conv-d", "Add a scratch pad", func(string) error { return nil })
	s.Require().NoError(err)
}

func (s *ConversationServiceTestSuite) TestChat_TicketsDraftIsStoredAsATable() {
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(s.draftingConv("tickets"), nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "Build the API"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.Anything, mock.Anything).Return(&draft.Result{Tickets: []draft.Ticket{
		{Title: "Add endpoint", Body: "Do it | now", AcceptanceCriteria: []string{"returns 201", "stores row"}},
		{Title: "Docs", Body: "Write", DependsOn: []string{"Add endpoint"}},
	}}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		if m.DraftKind != "tickets" {
			return false
		}
		got := draft.ParseProposedTickets(m.Content)
		return len(got) == 2 && got[0].Title == "Add endpoint" && len(got[0].AcceptanceCriteria) == 2 &&
			got[1].DependsOn[0] == "Add endpoint"
	})).Return(nil).Once()

	_, err := s.svc.Chat(context.Background(), "conv-d", "Build the API", func(string) error { return nil })
	s.Require().NoError(err)
}

func (s *ConversationServiceTestSuite) TestChat_DraftFailureSavesNothing() {
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(s.draftingConv("design"), nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Role == v1.MessageRole_MESSAGE_ROLE_USER
	})).Return(nil).Once()
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "x"},
	}, nil)
	s.drafter.On("Draft", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("ollama down"))

	_, err := s.svc.Chat(context.Background(), "conv-d", "x", func(string) error { return nil })
	s.ErrorContains(err, "ollama down")
}

func (s *ConversationServiceTestSuite) TestRegenerate_DiscardsLatestDraftAndDraftsAgain() {
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(s.draftingConv("design"), nil)
	s.msgRepo.On("ListByConversation", mock.Anything, "conv-d").Return([]*v1.Message{
		{Uuid: "p", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "What would you like to design?"},
		{Uuid: "u1", Role: v1.MessageRole_MESSAGE_ROLE_USER, Content: "A tracker"},
		{Uuid: "d1", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "# Design: old", DraftKind: "design"},
	}, nil)
	s.msgRepo.On("Delete", mock.Anything, "d1").Return(nil).Once()
	s.drafter.On("Draft", mock.Anything, mock.MatchedBy(func(r draft.DraftRequest) bool {
		return r.Source == "A tracker" && r.Current == ""
	}), mock.Anything).Return(&draft.Result{Body: "# Design: new"}, nil)
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Content == "# Design: new" && m.DraftKind == "design"
	})).Return(nil).Once()

	msg, err := s.svc.Regenerate(context.Background(), "conv-d", func(string) error { return nil })
	s.Require().NoError(err)
	s.Equal("# Design: new", msg.GetContent())
}

func (s *ConversationServiceTestSuite) TestRegenerate_RejectsNonDraftConversations() {
	s.convRepo.On("Get", mock.Anything, "conv-d").Return(s.draftingConv("research"), nil)

	_, err := s.svc.Regenerate(context.Background(), "conv-d", func(string) error { return nil })
	s.Error(err)
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
	s.T().Skip("cache temporarily disabled — re-enable after fixing per-query keying strategy")
	cache := mocks.NewMockResourceCache(s.T())
	svc := conversation.NewConversationService(
		s.convRepo, s.msgRepo, s.searcher, s.types, s.drafter, nil, s.llm, cache, zap.NewNop(), nil,
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
	s.T().Skip("cache temporarily disabled — re-enable after fixing per-query keying strategy")
	cache := mocks.NewMockResourceCache(s.T())
	svc := conversation.NewConversationService(
		s.convRepo, s.msgRepo, s.searcher, s.types, s.drafter, nil, s.llm, cache, zap.NewNop(), nil,
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
