package draft

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

var _ DraftService = (*draftService)(nil)

type draftService struct {
	gen    Generator
	logger *zap.Logger
}

// NewDraftService returns a DraftService that writes with gen.
func NewDraftService(gen Generator, logger *zap.Logger) DraftService {
	return &draftService{gen: gen, logger: logger}
}

func (s *draftService) Draft(ctx context.Context, req DraftRequest, emit func(token string) error) (*Result, error) {
	var system string
	switch req.Kind {
	case KindDiscovery:
		system = discoveryRules
	case KindDesign:
		system = designRules
	default:
		return nil, fmt.Errorf("draft kind must be discovery or design")
	}
	if strings.TrimSpace(req.Title) == "" && strings.TrimSpace(req.Source) == "" {
		return nil, fmt.Errorf("a title or source material is required")
	}

	s.logger.Info("drafting document", zap.Int("kind", int(req.Kind)), zap.String("title", req.Title))
	body, err := s.gen.Generate(ctx, system, userPrompt(req), emit)
	if err != nil {
		s.logger.Error("draft generation failed", zap.Error(err))
		return nil, err
	}
	body = stripThinking(body)

	res := &Result{Body: body}
	if req.Kind == KindDesign {
		res.Specs = ParseDomainObjects(body)
	}
	return res, nil
}

func userPrompt(req DraftRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(req.Title))
	if src := strings.TrimSpace(req.Source); src != "" {
		fmt.Fprintf(&b, "\nSource material:\n%s\n", src)
	}
	if cur := strings.TrimSpace(req.Current); cur != "" {
		fmt.Fprintf(&b, "\nCurrent draft (keep what is right, fill gaps, fix structure):\n%s\n", cur)
	}
	return b.String()
}

var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// stripThinking drops reasoning blocks some models emit even when asked
// not to, so they never land in a saved document.
func stripThinking(s string) string {
	return strings.TrimSpace(thinkBlock.ReplaceAllString(s, ""))
}

var operations = map[string]string{
	"create":    "create",
	"add":       "create",
	"new":       "create",
	"modify":    "modify",
	"change":    "modify",
	"update":    "modify",
	"deprecate": "deprecate",
	"remove":    "deprecate",
}

// ParseDomainObjects reads the "Domain objects" table of a design body into
// specs. Rows with an unrecognized operation or no name are skipped, as are
// dependencies on names not in the table — the table is model output, so it
// is read leniently and never trusted to be well formed.
func ParseDomainObjects(body string) []Spec {
	var rows [][]string
	inSection := false
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			inSection = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(t, "#")), "Domain objects")
			continue
		}
		if !inSection || !strings.HasPrefix(t, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(t, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) < 2 || strings.EqualFold(cells[0], "name") || strings.Trim(cells[0], "-: ") == "" {
			continue // header, separator, or empty row
		}
		rows = append(rows, cells)
	}

	var specs []Spec
	names := map[string]bool{}
	for _, cells := range rows {
		op, ok := operations[strings.ToLower(strings.Trim(cells[1], "*` "))]
		name := strings.Trim(cells[0], "*` ")
		if !ok || name == "" || names[name] {
			continue
		}
		names[name] = true
		sp := Spec{Name: name, Operation: op}
		if len(cells) > 2 {
			for _, dep := range strings.Split(cells[2], ",") {
				if dep = strings.Trim(dep, "*` "); dep != "" && dep != "-" && !strings.EqualFold(dep, "none") {
					sp.DependsOn = append(sp.DependsOn, dep)
				}
			}
		}
		specs = append(specs, sp)
	}
	for i := range specs {
		var deps []string
		for _, d := range specs[i].DependsOn {
			if names[d] && d != specs[i].Name {
				deps = append(deps, d)
			}
		}
		specs[i].DependsOn = deps
	}
	return specs
}
