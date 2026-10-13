package conversation

import (
	"fmt"
	"regexp"
	"strings"
)

// openQuestion is one row of a draft template's "Open Questions" table.
type openQuestion struct {
	Question string
	Leaning  string
}

var (
	openQuestionsHeading = regexp.MustCompile(`(?i)^#{1,6}\s*open\s+questions?\s*$`)
	tableSeparatorRow    = regexp.MustCompile(`^\|?[\s\-:|]+\|?$`)
)

// extractOpenQuestions finds a template's "Open Questions" section, if any,
// and returns its non-empty rows. Templates that have no such section (e.g.
// tickets) always yield nil, as does a section the model left empty.
func extractOpenQuestions(body string) []openQuestion {
	lines := strings.Split(body, "\n")

	start := -1
	for i, line := range lines {
		if openQuestionsHeading.MatchString(strings.TrimSpace(line)) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	var rows []string
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			break
		}
		if strings.HasPrefix(trimmed, "|") {
			rows = append(rows, trimmed)
		}
	}

	var questions []openQuestion
	seenSeparator := false
	for _, row := range rows {
		if !seenSeparator {
			if tableSeparatorRow.MatchString(row) {
				seenSeparator = true
			}
			continue
		}
		cells := splitTableRow(row)
		if len(cells) < 2 {
			continue
		}
		question := cells[len(cells)-2]
		leaning := cells[len(cells)-1]
		if question == "" || question == "-" {
			continue
		}
		questions = append(questions, openQuestion{Question: question, Leaning: leaning})
	}
	return questions
}

// splitTableRow splits a "| a | b | c |" markdown row into trimmed cells,
// dropping the empty leading/trailing cells the surrounding pipes produce.
func splitTableRow(row string) []string {
	parts := strings.Split(row, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// openQuestionsNudge builds a plain follow-up message surfacing a draft's
// open questions, inviting the user to answer (refining the draft) or just
// use Keep/Retry on the draft above as-is.
func openQuestionsNudge(questions []openQuestion) string {
	var b strings.Builder
	if len(questions) == 1 {
		b.WriteString("I had an open question while drafting this:\n\n")
	} else {
		b.WriteString("I had a couple of open questions while drafting this:\n\n")
	}
	for i, q := range questions {
		if q.Leaning != "" {
			fmt.Fprintf(&b, "%d. %s (leaning: %s)\n", i+1, q.Question, q.Leaning)
		} else {
			fmt.Fprintf(&b, "%d. %s\n", i+1, q.Question)
		}
	}
	b.WriteString("\nReply with answers to refine the draft, or just use Keep/Retry above if you're happy with it as-is.")
	return b.String()
}
