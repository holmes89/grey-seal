package grpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
	workgrpc "github.com/holmes89/grey-seal/lib/greyseal/worktype/grpc"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
)

func TestListWorkTypes_ReturnsSelectableTypes(t *testing.T) {
	reg, err := worktype.Load("")
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.Handle(servicesv1connect.NewWorkTypeServiceHandler(workgrpc.NewWorkTypeHandler(reg)))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := servicesv1connect.NewWorkTypeServiceClient(server.Client(), server.URL)

	resp, err := client.ListWorkTypes(context.Background(), connect.NewRequest(&services.ListWorkTypesRequest{}))
	require.NoError(t, err)

	var ids []string
	for _, wt := range resp.Msg.GetData() {
		ids = append(ids, wt.GetId())
	}
	require.Equal(t, []string{"discovery", "design", "tickets", "research", "other"}, ids)
	require.Equal(t, "What would you like to design?", resp.Msg.GetData()[1].GetPrompt())
	require.Equal(t, "design", resp.Msg.GetData()[1].GetDraftKind())
}
