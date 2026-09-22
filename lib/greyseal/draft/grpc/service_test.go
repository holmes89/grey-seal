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
	"github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DraftGRPCHandlerTestSuite struct {
	suite.Suite
	svc    *mocks.MockDraftService
	client servicesv1connect.DraftServiceClient
	server *httptest.Server
}

func TestDraftGRPCHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DraftGRPCHandlerTestSuite))
}

func (s *DraftGRPCHandlerTestSuite) SetupTest() {
	s.svc = mocks.NewMockDraftService(s.T())
	mux := http.NewServeMux()
	mux.Handle(servicesv1connect.NewDraftServiceHandler(grpchandler.NewDraftHandler(s.svc)))
	s.server = httptest.NewServer(mux)
	s.client = servicesv1connect.NewDraftServiceClient(s.server.Client(), s.server.URL)
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
