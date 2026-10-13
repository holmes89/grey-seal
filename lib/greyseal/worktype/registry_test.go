package worktype_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
)

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) { suite.Run(t, new(RegistrySuite)) }

func (s *RegistrySuite) write(dir, name, body string) {
	s.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

func (s *RegistrySuite) TestDefaults() {
	reg, err := worktype.Load("")
	s.Require().NoError(err)

	var ids []string
	for _, wt := range reg.List() {
		ids = append(ids, wt.ID)
	}
	s.Equal([]string{"discovery", "design", "tickets", "research", "other"}, ids, "proto is hidden")

	proto, ok := reg.Get("proto")
	s.True(ok)
	s.Equal("proto", proto.DraftKind)

	design, ok := reg.ByDraftKind("design")
	s.True(ok)
	s.Contains(design.System, "# Design: <Name>", "template placeholder is expanded")
	s.NotContains(design.System, "{{template}}")
}

func (s *RegistrySuite) TestMatch() {
	reg, err := worktype.Load("")
	s.Require().NoError(err)

	wt, ok := reg.Match("  DESIGN ")
	s.True(ok)
	s.Equal("design", wt.ID)

	wt, ok = reg.Match("something else")
	s.True(ok)
	s.Equal("other", wt.ID)

	_, ok = reg.Match("proto")
	s.False(ok, "hidden types are not selectable")
	_, ok = reg.Match("banana")
	s.False(ok)
}

func (s *RegistrySuite) TestDirOverridesAndAdds() {
	dir := s.T().TempDir()
	s.write(dir, "design.yaml", "id: design\nlabel: Blueprint\nprompt: Sketch it\ndraft_kind: design\n")
	s.write(dir, "extra.yaml", "id: review\nlabel: Review\n")

	reg, err := worktype.Load(dir)
	s.Require().NoError(err)

	design, _ := reg.Get("design")
	s.Equal("Blueprint", design.Label)
	_, ok := reg.Get("review")
	s.True(ok)

	var labels []string
	for _, wt := range reg.List() {
		labels = append(labels, wt.Label)
	}
	s.Equal([]string{"Discovery", "Blueprint", "Tickets", "Research", "Something else", "Review"}, labels, "overrides keep position, additions append")
}

func (s *RegistrySuite) TestErrors() {
	cases := map[string]string{
		"invalid yaml":  "id: [unterminated",
		"missing id":    "label: Nope\n",
		"missing label": "id: nope\n",
		"unknown field": "id: x\nlabel: X\nbogus: 1\n",
	}
	for name, body := range cases {
		dir := s.T().TempDir()
		s.write(dir, "bad.yaml", body)
		_, err := worktype.Load(dir)
		s.Error(err, name)
	}

	dir := s.T().TempDir()
	s.write(dir, "a.yaml", "id: dup\nlabel: A\n")
	s.write(dir, "b.yaml", "id: dup\nlabel: B\n")
	_, err := worktype.Load(dir)
	s.ErrorContains(err, "duplicate")

	_, err = worktype.Load(filepath.Join(dir, "missing"))
	s.Error(err)
}
