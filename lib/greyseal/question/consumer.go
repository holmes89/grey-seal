package question

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"google.golang.org/protobuf/proto"
)

type QuestionConsumer struct {
	consumer        base.Consumer[*entitiesv1.Question]
	questionservice QuestionService
}

func NewQuestionConsumer(
	consumer base.Consumer[*entitiesv1.Question],
	questionservice QuestionService,
) {
	con := &QuestionConsumer{
		consumer:        consumer,
		questionservice: questionservice,
	}
	go con.run()
}

func ConvertProto(data []byte) (*entitiesv1.Question, error) {
	var msg entitiesv1.Question
	log.Println("message", string(data))
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to process message: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *QuestionConsumer) run() {
	for i := range c.consumer.Read() {

		question := &entitiesv1.Question{
			Uuid:            uuid.New().String(),
			RoleDescription: i.RoleDescription,
			Content:         i.Content,
		}

		_, err := c.questionservice.Create(context.Background(), &servicesv1.CreateQuestionRequest{
			Data: question,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("question %s was imported\n", i.Uuid)
	}
}
