package conversation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/holmes89/grey-seal/lib/greyseal/conversation"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation/mocks"
	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type fakeRemora struct {
	designs, docs    []string // titles
	designNarwhalDoc string
	tickets          []string // "title|body"
	ticketDesign     string
	failTicketAt     int // 1-based; 0 = never
	err              error
}

func (f *fakeRemora) CreateDesign(_ context.Context, project, title, _, body, narwhalDocUUID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.designs = append(f.designs, project+"|"+title+"|"+body)
	f.designNarwhalDoc = narwhalDocUUID
	return "design-id", nil
}

func (f *fakeRemora) CreateDiscoveryDoc(_ context.Context, product, title, slug, _, body string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.docs = append(f.docs, product+"|"+title+"|"+slug)
	return "doc-id", nil
}

func (f *fakeRemora) CreateTicket(_ context.Context, _, design, title, body string) (string, error) {
	if f.failTicketAt > 0 && len(f.tickets)+1 == f.failTicketAt {
		return "", errors.New("remora down")
	}
	f.ticketDesign = design
	f.tickets = append(f.tickets, title+"|"+body)
	return fmt.Sprintf("t-%d", len(f.tickets)), nil
}

type KeepDraftSuite struct {
	suite.Suite
	convRepo *mocks.MockConversationRepository
	msgRepo  *mocks.MockMessageRepository
	remora   *fakeRemora
	svc      conversation.ConversationService
}

func TestKeepDraftSuite(t *testing.T) { suite.Run(t, new(KeepDraftSuite)) }

func (s *KeepDraftSuite) SetupTest() {
	s.convRepo = mocks.NewMockConversationRepository(s.T())
	s.msgRepo = mocks.NewMockMessageRepository(s.T())
	s.remora = &fakeRemora{}
	// No work-type registry by default: offerChain is then a no-op, so the
	// Keep-only tests don't need to mock the extra Create/Update calls it
	// would otherwise make. Tests of the chain offer itself build their own
	// service instance with a real registry (see offerChainSvc below).
	s.svc = conversation.NewConversationService(s.convRepo, s.msgRepo, nil, nil, nil, s.remora, nil, nil, zap.NewNop(), nil)
	s.convRepo.On("Get", mock.Anything, "c1").Return(&v1.Conversation{Uuid: "c1", Title: "My chat"}, nil).Maybe()
}

// offerChainSvc builds a service like s.svc but with a real work-type
// registry, so offerChain actually runs.
func (s *KeepDraftSuite) offerChainSvc() conversation.ConversationService {
	types, err := worktype.Load("")
	s.Require().NoError(err)
	return conversation.NewConversationService(s.convRepo, s.msgRepo, nil, types, nil, s.remora, nil, nil, zap.NewNop(), nil)
}

func (s *KeepDraftSuite) message(kind, content, kept string) {
	s.msgRepo.On("Get", mock.Anything, "m1").Return(&v1.Message{
		Uuid: "m1", ConversationUuid: "c1", Role: v1.MessageRole_MESSAGE_ROLE_ASSISTANT,
		Content: content, DraftKind: kind, KeptRef: kept,
	}, nil)
}

func (s *KeepDraftSuite) TestDesignIsWrittenAndRecorded() {
	s.message("design", "# Design: Shipment Tracker\n\n## Summary\nx", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.MatchedBy(func(m *v1.Message) bool {
		return m.KeptRef == "design:design-id"
	})).Return(nil).Once()

	msg, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj"})
	s.Require().NoError(err)
	s.Equal("design:design-id", msg.GetKeptRef())
	s.Require().Len(s.remora.designs, 1)
	s.Contains(s.remora.designs[0], "proj|Shipment Tracker|# Design: Shipment Tracker")
}

func (s *KeepDraftSuite) TestDesignLinksBackToTheDiscoveryDocItWasDraftedFrom() {
	s.message("design", "# Design: Shipment Tracker\n\n## Summary\nx", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.Anything).Return(nil).Once()

	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{
		ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj", NarwhalDocUUID: "doc-7",
	})
	s.Require().NoError(err)
	s.Equal("doc-7", s.remora.designNarwhalDoc)
}

func (s *KeepDraftSuite) TestKeepOffersChainWhenNextWorkTypeExists() {
	s.message("discovery", "# Feature Discovery: Notes - Attach notes to any entity\n\n## Overview", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.Anything).Return(nil).Once()
	s.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *v1.Message) bool {
		return m.Content == "Would you like me to move onto Design next?"
	})).Return(nil).Once()
	s.convRepo.On("Update", mock.Anything, "c1", mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.PendingChainOffer == "design" &&
			c.PendingChainOfferSource == "# Feature Discovery: Notes - Attach notes to any entity\n\n## Overview"
	})).Return(nil).Once()

	_, err := s.offerChainSvc().KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProductUUID: "prod"})
	s.Require().NoError(err)
}

func (s *KeepDraftSuite) TestKeepDoesNotOfferChainForATerminalWorkType() {
	s.message("tickets", "| Title | Body | Acceptance criteria | Depends on |\n| --- | --- | --- | --- |\n| T | B | c | |", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.Anything).Return(nil).Once()
	// No further Create/Update calls are mocked — if offerChain tried to
	// persist anything for a terminal work type, this strict mock would
	// reject the unexpected call and the test would fail.

	_, err := s.offerChainSvc().KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj"})
	s.Require().NoError(err)
}

func (s *KeepDraftSuite) TestDiscoveryUsesProductAndSlug() {
	s.message("discovery", "# Feature Discovery: Notes - Attach notes to any entity\n\n## Overview", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.Anything).Return(nil).Once()

	msg, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProductUUID: "prod"})
	s.Require().NoError(err)
	s.Equal("doc:doc-id", msg.GetKeptRef())
	s.Equal([]string{"prod|Notes|notes"}, s.remora.docs)
}

func (s *KeepDraftSuite) TestTicketsAreCreatedWithCriteriaAndDependenciesInTheBody() {
	s.message("tickets", "| Title | Body | Acceptance criteria | Depends on |\n| --- | --- | --- | --- |\n"+
		"| Add API | Build it | returns 201; stores row | |\n| Docs | Write them | | Add API |\n", "")
	s.msgRepo.On("Update", mock.Anything, "m1", mock.Anything).Return(nil).Once()

	msg, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj", DesignUUID: "d1"})
	s.Require().NoError(err)
	s.Equal("tickets:t-1,t-2", msg.GetKeptRef())
	s.Equal("d1", s.remora.ticketDesign)
	s.Equal("Add API|Build it\n\n## Acceptance criteria\n- returns 201\n- stores row", s.remora.tickets[0])
	s.Equal("Docs|Write them\n\nDepends on: Add API", s.remora.tickets[1])
}

func (s *KeepDraftSuite) TestPartialTicketFailureReportsProgressAndRecordsNothing() {
	s.remora.failTicketAt = 2
	s.message("tickets", "| Title | Body | Acceptance criteria | Depends on |\n| --- | --- | --- | --- |\n| A | a | | |\n| B | b | | |\n", "")

	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj"})
	s.ErrorContains(err, "1 of 2 created")
	s.msgRepo.AssertNotCalled(s.T(), "Update", mock.Anything, mock.Anything, mock.Anything)
}

func (s *KeepDraftSuite) TestRemoraFailureLeavesDraftUntouched() {
	s.remora.err = errors.New("remora down")
	s.message("design", "# Design: X", "")

	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj"})
	s.ErrorContains(err, "remora down")
	s.NotErrorIs(err, conversation.ErrKeepInvalid)
	s.msgRepo.AssertNotCalled(s.T(), "Update", mock.Anything, mock.Anything, mock.Anything)
}

func (s *KeepDraftSuite) TestInvalidRequests() {
	s.message("design", "# Design: X", "design:old")
	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "proj"})
	s.ErrorIs(err, conversation.ErrKeepInvalid, "already kept")
}

func (s *KeepDraftSuite) TestMissingTargetsAndWrongConversation() {
	s.message("design", "# Design: X", "")
	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1"})
	s.ErrorIs(err, conversation.ErrKeepInvalid, "design needs a project")

	_, err = s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "other", MessageUUID: "m1", ProjectUUID: "p"})
	s.ErrorIs(err, conversation.ErrKeepInvalid, "message belongs to another conversation")
}

func (s *KeepDraftSuite) TestPlainMessageIsNotKeepable() {
	s.message("", "just chat", "")
	_, err := s.svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "p"})
	s.ErrorIs(err, conversation.ErrKeepInvalid)
}

func (s *KeepDraftSuite) TestUnavailableWithoutRemora() {
	svc := conversation.NewConversationService(s.convRepo, s.msgRepo, nil, nil, nil, nil, nil, nil, zap.NewNop(), nil)
	_, err := svc.KeepDraft(context.Background(), conversation.KeepDraftInput{ConversationUUID: "c1", MessageUUID: "m1", ProjectUUID: "p"})
	s.ErrorIs(err, conversation.ErrKeepUnavailable)
}
