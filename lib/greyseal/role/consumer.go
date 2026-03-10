//go:build ignore

package role
import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/holmes89/archaea/base"
	entitiesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"google.golang.org/protobuf/proto"
)

type RoleConsumer struct {
	consumer    base.Consumer[*entitiesv1.Role]
	roleservice base.Service[*entitiesv1.Role]
}

func NewRoleConsumer(
	consumer base.Consumer[*entitiesv1.Role],
	roleservice base.Service[*entitiesv1.Role],
) {
	con := &RoleConsumer{
		consumer:    consumer,
		roleservice: roleservice,
	}
	go con.run()
}

func ConvertProto(data []byte) (*entitiesv1.Role, error) {
	var msg entitiesv1.Role
	log.Println("message", string(data))
	if err := proto.Unmarshal(data, &msg); err != nil {
		log.Printf("unable to process message: %s\n", err)
		return nil, err
	}
	return &msg, nil
}

func (c *RoleConsumer) run() {
	for i := range c.consumer.Read() {

		role := &entitiesv1.Role{
			Uuid: uuid.New().String(),
			Name: i.Name,
			SystemPrompt: i.SystemPrompt,
			CreatedAt: i.CreatedAt,
			
		}

		_, err := c.roleservice.Create(context.Background(), &servicesv1.CreateRoleRequest{
			Data: role,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("role %s was imported\n", i.Uuid)
	}
}

