package resource

import (
	"context"
	"log"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"google.golang.org/protobuf/proto"
)

type ResourceConsumer struct {
	consumer        base.Consumer[*Resource]
	resourceService ResourceService
}

func NewResourceConsumer(
	consumer base.Consumer[*Resource],
	resourceService ResourceService,
) {
	c := &ResourceConsumer{
		consumer:        consumer,
		resourceService: resourceService,
	}
	go c.run()
}

func ConvertProto(data []byte) (*Resource, error) {
	var msg Resource
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to unmarshal resource: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *ResourceConsumer) run() {
	for r := range c.consumer.Read() {
		if _, err := c.resourceService.Ingest(context.Background(), r); err != nil {
			log.Printf("failed to ingest resource %s: %v\n", r.Uuid, err)
			continue
		}
		log.Printf("ingested resource %s\n", r.Uuid)
	}
}
