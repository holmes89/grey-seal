package conversation

import (
	"testing"

	"github.com/holmes89/grey-seal/lib/greyseal/worktype"
)

func TestAcceptsChainOffer(t *testing.T) {
	types, err := worktype.Load("")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"plain yes", "yes", true},
		{"casual yep", "Yep!", false}, // punctuation defeats the exact switch match
		{"affirmative with case/space", "  Sure  ", true},
		{"offered type's id", "design", true},
		{"offered type's label, any case", "DESIGN", true},
		{"a different type's label", "Tickets", false},
		{"decline", "no thanks", false},
		{"unrelated reply", "actually, what about something else", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := acceptsChainOffer(tc.content, "design", types)
			if got != tc.want {
				t.Errorf("acceptsChainOffer(%q, \"design\") = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestAcceptsChainOffer_NilRegistryOnlyMatchesPlainAffirmatives(t *testing.T) {
	if !acceptsChainOffer("yes", "design", nil) {
		t.Error("expected a plain affirmative to match with no registry")
	}
	if acceptsChainOffer("design", "design", nil) {
		t.Error("expected a type-name match to require a registry")
	}
}
