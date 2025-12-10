package question

import (
	"context"

	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type QuestionService interface {
	List(con context.Context, lis base.ListRequest) (base.ListResponse[*Question], error)
	Get(con context.Context, get base.GetRequest[*Question]) (base.GetResponse[*Question], error)
	Create(con context.Context, cre base.CreateRequest[*Question]) (base.CreateResponse[*Answer], error)
}

type QueryResult struct {
	ResourceUUID string
	Content      string
}

type Querier interface {
	Query(ctx context.Context, query string, limit int) ([]QueryResult, error)
}
