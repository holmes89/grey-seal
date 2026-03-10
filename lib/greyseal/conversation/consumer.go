//go:build ignore

package conversation
import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"google.golang.org/protobuf/proto"
)

type MessageConsumer struct {
	consumer    base.Consumer[*entitiesv1.Message]
	messageservice base.Service[*entitiesv1.Message]
}

func NewMessageConsumer(
	consumer base.Consumer[*entitiesv1.Message],
	messageservice base.Service[*entitiesv1.Message],
) {
	con := &MessageConsumer{
		consumer:    consumer,
		messageservice: messageservice,
	}
	go con.run()
}

func ConvertProto(data []byte) (*entitiesv1.Message, error) {
	var msg entitiesv1.Message
	log.Println("message", string(data))
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to process message: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *MessageConsumer) run() {
	for i := range c.consumer.Read() {

		message := &entitiesv1.Message{
			Uuid: uuid.New().String(),
			ConversationUuid: i.ConversationUuid,
			Role: i.Role,
			Content: i.Content,
			ResourceUuids: i.ResourceUuids,
			Feedback: i.Feedback,
			CreatedAt: i.CreatedAt,
			
		}

		_, err := c.messageservice.Create(context.Background(), &servicesv1.CreateMessageRequest{
			Data: message,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("message %s was imported\n", i.Uuid)
	}
}

type ConversationConsumer struct {
	consumer    base.Consumer[*entitiesv1.Conversation]
	conversationservice base.Service[*entitiesv1.Conversation]
}

func NewConversationConsumer(
	consumer base.Consumer[*entitiesv1.Conversation],
	conversationservice base.Service[*entitiesv1.Conversation],
) {
	con := &ConversationConsumer{
		consumer:    consumer,
		conversationservice: conversationservice,
	}
	go con.run()
}

func ConvertProto(data []byte) (*entitiesv1.Conversation, error) {
	var msg entitiesv1.Conversation
	log.Println("message", string(data))
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to process message: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *ConversationConsumer) run() {
	for i := range c.consumer.Read() {

		conversation := &entitiesv1.Conversation{
			Uuid: uuid.New().String(),
			Title: i.Title,
			RoleUuid: i.RoleUuid,
			ResourceUuids: i.ResourceUuids,
			Summary: i.Summary,
			Messages: i.Messages,
			CreatedAt: i.CreatedAt,
			UpdatedAt: i.UpdatedAt,
			
		}

		_, err := c.conversationservice.Create(context.Background(), &servicesv1.CreateConversationRequest{
			Data: conversation,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("conversation %s was imported\n", i.Uuid)
	}
}

