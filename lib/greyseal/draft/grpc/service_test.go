package grpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	grpchandler "github.com/holmes89/grey-seal/lib/greyseal/draft/grpc"
	"github.com/holmes89/grey-seal/lib/greyseal/draft/mocks"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesconnect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DraftGRPCHandlerTestSuite struct {
	suite.Suite
	svc    *mocks.MockDraftService
	client servicesconnect.DraftServiceClient
	server *httptest.Server
}

func TestDraftGRPCHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DraftGRPCHandlerTestSuite))
}

func (s *DraftGRPCHandlerTestSuite) SetupTest() {
	s.svc = mocks.NewMockDraftService(s.T())
	mux := http.NewServeMux()
	mux.Handle(servicesconnect.NewDraftServiceHandler(grpchandler.NewDraftHandler(s.svc)))
	s.server = httptest.NewServer(mux)
	s.client = servicesconnect.NewDraftServiceClient(s.server.Client(), s.server.URL)
}

func (s *DraftGRPCHandlerTestSuite) TearDownTest() {
	s.server.Close()
}

func (s *DraftGRPCHandlerTestSuite) TestStreamsTokensThenFinalChunk() {
	s.svc.On("Draft", mock.Anything,
		draft.DraftRequest{Kind: draft.KindDesign, Title: "T", Source: "src", Current: "cur"},
		mock.Anything,
	).Run(func(args mock.Arguments) {
		emit := args.Get(2).(func(string) error)
		s.Require().NoError(emit("# Design"))
		s.Require().NoError(emit(": T"))
	}).Return(&draft.Result{
		Body:  "# Design: T",
		Specs: []draft.Spec{{Name: "Note", Operation: "create", DependsOn: []string{"Owner"}}},
	}, nil)

	stream, err := s.client.DraftDocument(context.Background(), connect.NewRequest(&services.DraftDocumentRequest{
		Kind: services.DraftKind_DRAFT_KIND_DESIGN, Title: "T", Source: "src", Current: "cur",
	}))
	s.Require().NoError(err)
	var chunks []*services.DraftDocumentChunk
	for stream.Receive() {
		chunks = append(chunks, stream.Msg())
	}
	s.Require().NoError(stream.Err())
	s.Require().Len(chunks, 3)
	s.Equal("# Design", chunks[0].GetToken())
	s.Equal(": T", chunks[1].GetToken())
	s.True(chunks[2].GetDone())
	s.Equal("# Design: T", chunks[2].GetBody())
	s.Require().Len(chunks[2].GetSpecs(), 1)
	s.Equal("Note", chunks[2].GetSpecs()[0].GetName())
	s.Equal([]string{"Owner"}, chunks[2].GetSpecs()[0].GetDependsOn())
}

func (s *DraftGRPCHandlerTestSuite) TestUnspecifiedKindIsInvalid() {
	stream, err := s.client.DraftDocument(context.Background(), connect.NewRequest(&services.DraftDocumentRequest{Title: "T"}))
	s.Require().NoError(err)
	for stream.Receive() {
	}
	s.Equal(connect.CodeInvalidArgument, connect.CodeOf(stream.Err()))
}

func (s *DraftGRPCHandlerTestSuite) TestProtoKindPassesPackage() {
	s.svc.On("Draft", mock.Anything,
		draft.DraftRequest{Kind: draft.KindProto, Title: "Shipment", ProtoPackage: "shipping"},
		mock.Anything,
	).Return(&draft.Result{Body: "syntax = \"proto3\";\n"}, nil)

	stream, err := s.client.DraftDocument(context.Background(), connect.NewRequest(&services.DraftDocumentRequest{
		Kind: services.DraftKind_DRAFT_KIND_PROTO, Title: "Shipment", ProtoPackage: "shipping",
	}))
	s.Require().NoError(err)
	var last *services.DraftDocumentChunk
	for stream.Receive() {
		last = stream.Msg()
	}
	s.Require().NoError(stream.Err())
	s.True(last.GetDone())
	s.Equal("syntax = \"proto3\";\n", last.GetBody())
}
