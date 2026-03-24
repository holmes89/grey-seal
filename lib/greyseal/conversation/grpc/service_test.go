package grpc_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	grpchandler "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/archaea/base"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConversationGRPCHandlerTestSuite struct {
	suite.Suite
	svc     *mocks.MockConversationService
	handler *grpchandler.ConversationHandler
}

func (s *ConversationGRPCHandlerTestSuite) SetupTest() {
	s.svc = mocks.NewMockConversationService(s.T())
	s.handler = grpchandler.NewConversationHandler(s.svc)
}

func (s *ConversationGRPCHandlerTestSuite) TestListConversations() {
	convs := []*v1.Conversation{{Uuid: "c1", Title: "Chat"}}
	listResp := &base.ListGenericResponse[*v1.Conversation]{Data: convs, Count: 1}
	s.svc.On("List", mock.Anything, mock.Anything).Return(listResp, nil)

	count := int32(1)
	req := connect.NewRequest(&services.ListConversationsRequest{Count: &count})
	resp, err := s.handler.ListConversations(context.Background(), req)
	s.Require().NoError(err)
	s.Len(resp.Msg.GetData(), 1)
	s.Equal("c1", resp.Msg.GetData()[0].GetUuid())
}

func (s *ConversationGRPCHandlerTestSuite) TestGetConversation() {
	conv := &v1.Conversation{Uuid: "g1", Title: "Test"}
	getResp := &base.GetGenericResponse[*v1.Conversation]{Data: conv}
	s.svc.On("Get", mock.Anything, mock.Anything).Return(getResp, nil)

	req := connect.NewRequest(&services.GetConversationRequest{Uuid: "g1"})
	resp, err := s.handler.GetConversation(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("g1", resp.Msg.GetData().GetUuid())
}

func (s *ConversationGRPCHandlerTestSuite) TestCreateConversation() {
	created := &v1.Conversation{Uuid: "new-1", Title: "New Chat"}
	s.svc.On("Create", mock.Anything, mock.MatchedBy(func(c *v1.Conversation) bool {
		return c.Title == "New Chat"
	})).Return(created, nil)

	req := connect.NewRequest(&services.CreateConversationRequest{Title: "New Chat"})
	resp, err := s.handler.CreateConversation(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("new-1", resp.Msg.GetData().GetUuid())
}

func (s *ConversationGRPCHandlerTestSuite) TestUpdateConversation() {
	updated := &v1.Conversation{Uuid: "u1", Title: "Updated"}
	s.svc.On("Update", mock.Anything, "u1", mock.Anything).Return(updated, nil)

	title := "Updated"
	req := connect.NewRequest(&services.UpdateConversationRequest{Uuid: "u1", Title: &title})
	resp, err := s.handler.UpdateConversation(context.Background(), req)
	s.Require().NoError(err)
	s.Equal("Updated", resp.Msg.GetData().GetTitle())
}

func (s *ConversationGRPCHandlerTestSuite) TestDeleteConversation() {
	s.svc.On("Delete", mock.Anything, "d1").Return(nil)

	req := connect.NewRequest(&services.DeleteConversationRequest{Uuid: "d1"})
	_, err := s.handler.DeleteConversation(context.Background(), req)
	s.Require().NoError(err)
}

func (s *ConversationGRPCHandlerTestSuite) TestSubmitFeedback() {
	s.svc.On("SubmitFeedback", mock.Anything, "msg-1", int32(1)).Return(nil)

	req := connect.NewRequest(&services.SubmitFeedbackRequest{MessageUuid: "msg-1", Feedback: 1})
	_, err := s.handler.SubmitFeedback(context.Background(), req)
	s.Require().NoError(err)
}

func (s *ConversationGRPCHandlerTestSuite) TestDeleteConversation_Error() {
	s.svc.On("Delete", mock.Anything, "bad").Return(errors.New("not found"))

	req := connect.NewRequest(&services.DeleteConversationRequest{Uuid: "bad"})
	_, err := s.handler.DeleteConversation(context.Background(), req)
	s.Require().Error(err)
}

func TestConversationGRPCHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ConversationGRPCHandlerTestSuite))
}
