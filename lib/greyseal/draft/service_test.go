package draft_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/holmes89/grey-seal/lib/greyseal/draft"
	"github.com/holmes89/grey-seal/lib/greyseal/draft/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

const designBody = `# Design: Offline notes

## Summary
Notes save locally and sync later.

## Domain objects
| Name | Operation | Depends on |
| --- | --- | --- |
| NoteDraft | create | |
| SyncQueue | Create | NoteDraft |
| Note | modify | NoteDraft, Ghost |
| **Attachment** | remove | none |
| Bogus | explode | |

## Approach
Local-first.`

type DraftServiceSuite struct {
	suite.Suite
	gen *mocks.MockGenerator
	svc draft.DraftService
}

func TestDraftServiceSuite(t *testing.T) {
	suite.Run(t, new(DraftServiceSuite))
}

func (s *DraftServiceSuite) SetupTest() {
	s.gen = mocks.NewMockGenerator(s.T())
	s.svc = draft.NewDraftService(s.gen, zap.NewNop())
}

func (s *DraftServiceSuite) TestDiscovery_UsesDiscoveryTemplateAndStreams() {
	var tokens []string
	s.gen.On("Generate", mock.Anything,
		mock.MatchedBy(func(sys string) bool {
			return strings.Contains(sys, "# Feature Discovery:") && strings.Contains(sys, "| # | Question | Leaning |")
		}),
		mock.MatchedBy(func(p string) bool {
			return strings.Contains(p, "Title: Offline notes") && strings.Contains(p, "techs lose signal")
		}),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		emit := args.Get(3).(func(string) error)
		s.Require().NoError(emit("# Feature"))
		s.Require().NoError(emit(" Discovery"))
	}).Return("# Feature Discovery", nil)

	res, err := s.svc.Draft(context.Background(), draft.DraftRequest{
		Kind: draft.KindDiscovery, Title: "Offline notes", Source: "Field techs lose signal",
	}, func(t string) error { tokens = append(tokens, t); return nil })

	s.Require().NoError(err)
	s.Equal([]string{"# Feature", " Discovery"}, tokens)
	s.Equal("# Feature Discovery", res.Body)
	s.Empty(res.Specs, "discovery docs never carry specs")
}

func (s *DraftServiceSuite) TestDesign_ParsesSpecsFromDomainObjects() {
	s.gen.On("Generate", mock.Anything,
		mock.MatchedBy(func(sys string) bool { return strings.Contains(sys, "# Design: <Name>") }),
		mock.Anything, mock.Anything,
	).Return(designBody, nil)

	res, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindDesign, Title: "Offline notes"}, nil)

	s.Require().NoError(err)
	s.Equal([]draft.Spec{
		{Name: "NoteDraft", Operation: "create"},
		{Name: "SyncQueue", Operation: "create", DependsOn: []string{"NoteDraft"}},
		{Name: "Note", Operation: "modify", DependsOn: []string{"NoteDraft"}},
		{Name: "Attachment", Operation: "deprecate"},
	}, res.Specs)
}

func (s *DraftServiceSuite) TestCurrentDraftIsPassedToImproveOn() {
	s.gen.On("Generate", mock.Anything, mock.Anything,
		mock.MatchedBy(func(p string) bool { return strings.Contains(p, "Current draft") && strings.Contains(p, "old body") }),
		mock.Anything,
	).Return("new body", nil)

	_, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindDiscovery, Title: "T", Current: "old body"}, nil)
	s.Require().NoError(err)
}

func (s *DraftServiceSuite) TestStripsThinkingBlocks() {
	s.gen.On("Generate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("<think>\nplanning...\n</think>\n\n# Feature Discovery: X", nil)

	res, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindDiscovery, Title: "X"}, nil)
	s.Require().NoError(err)
	s.Equal("# Feature Discovery: X", res.Body)
}

func (s *DraftServiceSuite) TestRejectsUnknownKindAndEmptyInput() {
	_, err := s.svc.Draft(context.Background(), draft.DraftRequest{Title: "X"}, nil)
	s.Error(err)
	_, err = s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindDesign}, nil)
	s.Error(err)
}

func (s *DraftServiceSuite) TestGeneratorErrorIsReturned() {
	s.gen.On("Generate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("ollama down"))
	_, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindDesign, Title: "X"}, nil)
	s.EqualError(err, "ollama down")
}

func TestParseDomainObjects_NoSectionOrTable(t *testing.T) {
	suite.Run(t, new(parseSuite))
}

type parseSuite struct{ suite.Suite }

func (s *parseSuite) TestNoSection() {
	s.Empty(draft.ParseDomainObjects("# Design: X\n\n## Summary\n| Name | Operation |\n| Foo | create |"))
}

func (s *parseSuite) TestSectionEndsAtNextHeading() {
	body := "## Domain objects\n| Name | Operation | Depends on |\n| --- | --- | --- |\n| Foo | create | |\n## Approach\n| Bar | create | |"
	s.Equal([]draft.Spec{{Name: "Foo", Operation: "create"}}, draft.ParseDomainObjects(body))
}

func (s *parseSuite) TestDuplicateNamesKeepFirst() {
	body := "## Domain objects\n| Foo | create | |\n| Foo | modify | |"
	s.Equal([]draft.Spec{{Name: "Foo", Operation: "create"}}, draft.ParseDomainObjects(body))
}

func (s *DraftServiceSuite) TestProto_UsesProtoRulesPackageAndStripsFences() {
	s.gen.On("Generate", mock.Anything,
		mock.MatchedBy(func(sys string) bool {
			return strings.Contains(sys, "Protocol Buffers") && strings.Contains(sys, "string uuid = 1;")
		}),
		mock.MatchedBy(func(p string) bool {
			return strings.Contains(p, "Domain object: Shipment") && strings.Contains(p, "Package: shipping") &&
				strings.Contains(p, "design body")
		}),
		mock.Anything,
	).Return("```proto\nsyntax = \"proto3\";\npackage shipping;\n\nmessage Shipment {\n  string uuid = 1;\n}\n```", nil)

	res, err := s.svc.Draft(context.Background(), draft.DraftRequest{
		Kind: draft.KindProto, Title: "Shipment", ProtoPackage: "shipping", Source: "design body",
	}, nil)

	s.Require().NoError(err)
	s.Equal("syntax = \"proto3\";\npackage shipping;\n\nmessage Shipment {\n  string uuid = 1;\n}\n", res.Body)
	s.Empty(res.Specs)
}

func (s *DraftServiceSuite) TestProto_ValidatesPackageAndName() {
	_, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindProto, Title: "Shipment", ProtoPackage: "Bad-Pkg"}, nil)
	s.ErrorContains(err, "proto package")
	_, err = s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindProto, Title: "shipment line", ProtoPackage: "shipping"}, nil)
	s.ErrorContains(err, "PascalCase")
}

func (s *DraftServiceSuite) TestProto_AddsMissingTimestampImport() {
	s.gen.On("Generate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("syntax = \"proto3\";\npackage notes;\n\nmessage Revision {\n  string uuid = 1;\n  google.protobuf.Timestamp created_at = 2;\n}\n", nil)

	res, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindProto, Title: "Revision", ProtoPackage: "notes"}, nil)
	s.Require().NoError(err)
	s.Equal("syntax = \"proto3\";\npackage notes;\n\nimport \"google/protobuf/timestamp.proto\";\n\nmessage Revision {\n  string uuid = 1;\n  google.protobuf.Timestamp created_at = 2;\n}\n", res.Body)
}

func (s *DraftServiceSuite) TestProto_KeepsExistingImportAndPlainProtos() {
	withImport := "syntax = \"proto3\";\npackage notes;\n\nimport \"google/protobuf/timestamp.proto\";\n\nmessage R {\n  string uuid = 1;\n  google.protobuf.Timestamp at = 2;\n}\n"
	plain := "syntax = \"proto3\";\npackage notes;\n\nmessage R {\n  string uuid = 1;\n}\n"
	for _, body := range []string{withImport, plain} {
		s.SetupTest()
		s.gen.On("Generate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(body, nil)
		res, err := s.svc.Draft(context.Background(), draft.DraftRequest{Kind: draft.KindProto, Title: "R", ProtoPackage: "notes"}, nil)
		s.Require().NoError(err)
		s.Equal(body, res.Body)
	}
}
