package question

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type QuestionRepository interface {
	Create(context.Context, *Question) error
	Update(context.Context, string, *Question) error
	Delete(context.Context, string) error
	Get(context.Context, string) (*Question, error)
	List(context.Context, string, uint, map[string][]any) ([]*Question, error)
	SaveResponse(ctx context.Context, questionUUID, response string, references []string) error
}

var _ base.Entity = (*Question)(nil)
var _ base.Repository[*Question] = (QuestionRepository)(nil)
