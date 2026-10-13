// Package worktype defines the kinds of work a chat can be about (design,
// discovery, tickets, ...). Work types are yaml documents: a set of defaults
// is embedded in the binary and a directory of yaml files can add to or
// override them without a code change.
package worktype

// WorkType is one selectable kind of work.
type WorkType struct {
	// ID is the stable identifier stored on a conversation, e.g. "design".
	ID string `yaml:"id"`
	// Label is what the welcome message shows the user, e.g. "Design".
	Label string `yaml:"label"`
	// Prompt is sent after the user picks this type, asking for their input.
	Prompt string `yaml:"prompt"`
	// System is the system prompt. A "{{template}}" placeholder is replaced
	// with Template when the work type is loaded.
	System string `yaml:"system"`
	// Template is the document skeleton the model is held to.
	Template string `yaml:"template"`
	// DraftKind names the draft the type produces: discovery, design,
	// tickets or proto. Empty means the type is plain chat.
	DraftKind string `yaml:"draft_kind"`
	// NextWorkType is the id of the work type offered as a next step after
	// a draft of this type is kept (e.g. "design" for discovery). Empty
	// means no chain is offered.
	NextWorkType string `yaml:"next_work_type"`
	// UseRAG marks plain-chat types that should search the knowledge base.
	UseRAG bool `yaml:"use_rag"`
	// Hidden types are loadable by ID but not offered in the welcome.
	Hidden bool `yaml:"hidden"`
}

// Registry looks up loaded work types.
type Registry interface {
	// Get returns the work type with the given ID.
	Get(id string) (WorkType, bool)
	// ByDraftKind returns the work type that drafts the given kind.
	ByDraftKind(kind string) (WorkType, bool)
	// Match resolves a user's reply to a selectable work type by ID or
	// label, ignoring case and surrounding space.
	Match(reply string) (WorkType, bool)
	// List returns the selectable (non-hidden) work types in load order.
	List() []WorkType
}

// Pending is the conversation work type while the welcome message is
// waiting for the user to pick one. It is never defined in yaml.
const Pending = "pending"
