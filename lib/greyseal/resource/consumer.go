package resource

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"google.golang.org/protobuf/proto"
)

type ResourceConsumer struct {
	consumer        base.Consumer[*entitiesv1.Resource]
	resourceservice base.Service[*entitiesv1.Resource]
}

func NewResourceConsumer(
	consumer base.Consumer[*entitiesv1.Resource],
	resourceservice base.Service[*entitiesv1.Resource],
) {
	con := &ResourceConsumer{
		consumer:        consumer,
		resourceservice: resourceservice,
	}
	go con.run()
}

func ConvertProto(data []byte) (*entitiesv1.Resource, error) {
	var msg entitiesv1.Resource
	log.Println("message", string(data))
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to process message: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *ResourceConsumer) run() {
	for i := range c.consumer.Read() {

		resource := &entitiesv1.Resource{
			Uuid:      uuid.New().String(),
			CreatedAt: i.CreatedAt,
			Service:   i.Service,
			Entity:    i.Entity,
			Source:    i.Source,
			Path:      i.Path,
		}

		_, err := c.resourceservice.Create(context.Background(), &servicesv1.CreateResourceRequest{
			Data: resource,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("resource %s was imported\n", i.Uuid)
	}
}
