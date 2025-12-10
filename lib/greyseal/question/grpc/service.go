package grpc

import (
	"context"
	"log"

	"connectrpc.com/connect"
	"github.com/holmes89/archaea/base"

	"github.com/holmes89/grey-seal/lib/greyseal/question"
	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicev1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
)

type QuestionService struct {
	servicev1.UnimplementedQuestionServiceServer
	svc question.QuestionService
}

func NewQuestionService(svc question.QuestionService, publisher base.Producer[*entitiesv1.Question]) *QuestionService {
	return &QuestionService{
		svc: svc,
	}
}

func (s *QuestionService) ListQuestions(ctx context.Context, req *connect.Request[servicev1.ListQuestionsRequest]) (*connect.Response[servicev1.ListQuestionsResponse], error) {
	data, err := s.svc.List(ctx, req.Msg)
	if err != nil {
		log.Printf("Error from service: %v", err)
		return nil, err
	}

	log.Printf("Service returned %d questions", len(data.GetData()))
	resp := &servicev1.ListQuestionsResponse{
		Data:   data.GetData(),
		Cursor: data.GetCursor(),
		Count:  data.GetCount(),
	}
	connectResp := connect.NewResponse(resp)
	return connectResp, nil
}

func (s *QuestionService) GetQuestion(ctx context.Context, req *connect.Request[servicev1.GetQuestionRequest]) (*connect.Response[servicev1.GetQuestionResponse], error) {
	data, err := s.svc.Get(ctx, req.Msg)
	if err != nil {
		log.Printf("Error from service: %v", err)
		return nil, err
	}
	resp := &servicev1.GetQuestionResponse{
		Data: data.GetData(),
	}
	return connect.NewResponse(resp), nil
}

func (s *QuestionService) CreateQuestion(ctx context.Context, req *connect.Request[servicev1.CreateQuestionRequest]) (*connect.Response[servicev1.CreateQuestionResponse], error) {
	e, err := s.svc.Create(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	resp := req.Msg.Data
	resp.Uuid = e.GetData().GetUuid()
	return connect.NewResponse(&servicev1.CreateQuestionResponse{
		Data: e.GetData(),
	}), nil
}
