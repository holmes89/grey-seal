package conversation

import (
	"strings"
	"testing"
)

func TestExtractOpenQuestions(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []openQuestion
	}{
		{
			name: "design template, two questions",
			body: "# Design: X\n\n## Summary\nSomething.\n\n## Open questions\n" +
				"| # | Question | Leaning |\n| --- | --- | --- |\n" +
				"| 1 | How should X behave? | Neutral |\n" +
				"| 2 | What about Y? | Leaning towards yes |\n",
			want: []openQuestion{
				{Question: "How should X behave?", Leaning: "Neutral"},
				{Question: "What about Y?", Leaning: "Leaning towards yes"},
			},
		},
		{
			name: "discovery template, capitalized heading",
			body: "# Feature Discovery: X\n\n## Open Questions\n" +
				"| # | Question | Leaning |\n| --- | --- | --- |\n" +
				"| 1 | Who is this for? | Neutral |\n" +
				"## Success Metrics\nSome text.\n",
			want: []openQuestion{{Question: "Who is this for?", Leaning: "Neutral"}},
		},
		{
			name: "no open questions section",
			body: "| Title | Body | Acceptance criteria | Depends on |\n| --- | --- | --- | --- |\n" +
				"| Add endpoint | Do it | returns 201 | |\n",
			want: nil,
		},
		{
			name: "empty open questions section",
			body: "# Design: X\n\n## Open questions\n| # | Question | Leaning |\n| --- | --- | --- |\n",
			want: nil,
		},
		{
			name: "placeholder dash is skipped",
			body: "## Open questions\n| # | Question | Leaning |\n| --- | --- | --- |\n| 1 | - | - |\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractOpenQuestions(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d questions, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("question %d: got %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}

func TestOpenQuestionsNudge(t *testing.T) {
	single := openQuestionsNudge([]openQuestion{{Question: "How should X behave?", Leaning: "Neutral"}})
	if got := "I had an open question while drafting this"; !strings.Contains(single, got) {
		t.Errorf("singular nudge missing %q: %s", got, single)
	}
	if !strings.Contains(single, "How should X behave? (leaning: Neutral)") {
		t.Errorf("nudge missing question/leaning: %s", single)
	}
	if !strings.Contains(single, "Reply with answers to refine the draft, or just use Keep/Retry above") {
		t.Errorf("nudge missing call to action: %s", single)
	}

	plural := openQuestionsNudge([]openQuestion{
		{Question: "A?", Leaning: "Neutral"},
		{Question: "B?", Leaning: ""},
	})
	if got := "I had a couple of open questions while drafting this"; !strings.Contains(plural, got) {
		t.Errorf("plural nudge missing %q: %s", got, plural)
	}
	if !strings.Contains(plural, "1. A? (leaning: Neutral)") || !strings.Contains(plural, "2. B?\n") {
		t.Errorf("nudge did not number/format both questions: %s", plural)
	}
}
