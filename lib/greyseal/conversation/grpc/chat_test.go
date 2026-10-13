package grpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	grpchandler "github.com/holmes89/grey-seal/lib/greyseal/conversation/grpc"
	"github.com/holmes89/grey-seal/lib/greyseal/conversation/mocks"
	v1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func chatClient(t *testing.T, svc *mocks.MockConversationService) servicesv1connect.ConversationServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(servicesv1connect.NewConversationServiceHandler(grpchandler.NewConversationHandler(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return servicesv1connect.NewConversationServiceClient(server.Client(), server.URL)
}

func drain(t *testing.T, stream *connect.ServerStreamForClient[services.ChatResponse]) (tokens string, final *v1.Message) {
	t.Helper()
	for stream.Receive() {
		tokens += stream.Msg().GetToken()
		if m := stream.Msg().GetFinalMessage(); m != nil {
			final = m
		}
	}
	require.NoError(t, stream.Err())
	return tokens, final
}

func TestChat_StreamsTokensAndFinalMessage(t *testing.T) {
	svc := mocks.NewMockConversationService(t)
	svc.On("Chat", mock.Anything, "c1", "hello", mock.Anything).
		Run(func(args mock.Arguments) { _ = args.Get(3).(func(string) error)("hi") }).
		Return(&v1.Message{Uuid: "m1", Content: "hi"}, nil)

	stream, err := chatClient(t, svc).Chat(context.Background(), connect.NewRequest(&services.ChatRequest{ConversationUuid: "c1", Content: "hello"}))
	require.NoError(t, err)
	tokens, final := drain(t, stream)
	require.Equal(t, "hi", tokens)
	require.Equal(t, "m1", final.GetUuid())
}

func TestChat_RegenerateRoutesToRegenerate(t *testing.T) {
	svc := mocks.NewMockConversationService(t)
	svc.On("Regenerate", mock.Anything, "c1", mock.Anything).
		Run(func(args mock.Arguments) { _ = args.Get(2).(func(string) error)("again") }).
		Return(&v1.Message{Uuid: "m2", Content: "again", DraftKind: "design"}, nil)

	stream, err := chatClient(t, svc).Chat(context.Background(), connect.NewRequest(&services.ChatRequest{ConversationUuid: "c1", Regenerate: true}))
	require.NoError(t, err)
	tokens, final := drain(t, stream)
	require.Equal(t, "again", tokens)
	require.Equal(t, "design", final.GetDraftKind())
	svc.AssertNotCalled(t, "Chat", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
