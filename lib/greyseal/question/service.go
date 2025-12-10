package question

import (
	"context"
	"fmt"
	"strings"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/tmc/langchaingo/llms"
)

var _ QuestionService = (*questionService)(nil)

// var _ base.Service[*Question] = (*questionService)(nil) non standard

type questionService struct {
	questionRepo QuestionRepository
	client       llms.Model
	querier      Querier
}

func NewQuestionService(
	questionRepo QuestionRepository,
	querier Querier,
	client llms.Model,
) QuestionService {
	return &questionService{
		questionRepo: questionRepo,
		querier:      querier,
		client:       client,
	}
}

func (srv *questionService) List(con context.Context, lis base.ListRequest) (base.ListResponse[*Question], error) {
	data, err := srv.questionRepo.List(con, lis.GetCursor(), uint(lis.GetCount()), nil)
	return &base.ListGenericResponse[*Question]{
		Cursor: "",
		Count:  10,
		Data:   data,
	}, err
}

func (srv *questionService) Get(con context.Context, get base.GetRequest[*Question]) (base.GetResponse[*Question], error) {
	fmt.Println("get question", get.GetUuid())
	data, err := srv.questionRepo.Get(con, get.GetUuid())
	return &base.GetGenericResponse[*Question]{
		Data: data,
	}, err
}

func (srv *questionService) Create(con context.Context, cre base.CreateRequest[*Question]) (base.CreateResponse[*Answer], error) {
	fmt.Println("create question", cre.GetData())
	err := srv.questionRepo.Create(con, cre.GetData())
	if err != nil {
		return nil, err
	}
	contexts, err := srv.querier.Query(con, cre.GetData().GetContent(), 5)
	if err != nil {
		return nil, fmt.Errorf("failed to query contexts: %w", err)
	}

	// Build prompt with contexts
	promptBuilder := strings.Builder{}
	fmt.Fprintf(&promptBuilder, "You are going to take the role of %s.\n", cre.GetData().GetRoleDescription())
	fmt.Fprintf(&promptBuilder, "Based on the following contexts, please answer this question: %s\n\nContexts:\n", cre.GetData().GetContent())
	referencesSet := make(map[string]any)
	for i, ctx := range contexts {
		fmt.Fprintf(&promptBuilder, "%d. %s\n", i+1, ctx.Content)
		referencesSet[ctx.ResourceUUID] = nil
	}
	var references []string
	for ref := range referencesSet {
		references = append(references, ref)
	}

	// Generate answer using LLM
	response, err := srv.client.GenerateContent(con, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: promptBuilder.String()},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	// Extract text from response
	var answers strings.Builder
	for _, choice := range response.Choices {
		answers.WriteString(choice.Content)
		answers.WriteString("\n")
	}
	err = srv.questionRepo.SaveResponse(con, cre.GetData().GetUuid(), answers.String(), references)
	if err != nil {
		return nil, fmt.Errorf("failed to save response: %w", err)
	}

	return &base.CreateGenericResponse[*Answer]{
		Data: &Answer{
			Uuid:       cre.GetData().GetUuid(),
			Message:    answers.String(),
			References: references,
		},
	}, err
}
