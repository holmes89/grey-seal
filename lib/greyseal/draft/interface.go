package draft

import "context"

// Kind is the planning document being drafted.
type Kind int

const (
	KindUnspecified Kind = iota
	KindDiscovery
	KindDesign
	// KindProto drafts one domain object's .proto (Title is the Spec name).
	KindProto
)

// DraftService writes first drafts of planning documents in the house
// templates. It stores nothing: callers review a draft and save it to the
// service that owns the document.
type DraftService interface {
	// Draft generates a document, calling emit with each piece of markdown as
	// it arrives. Returning an error from emit aborts generation.
	Draft(ctx context.Context, req DraftRequest, emit func(token string) error) (*Result, error)
}

// Generator produces text from a system and user prompt, streaming it
// through emit. Implemented by an adapter over the Ollama client.
type Generator interface {
	Generate(ctx context.Context, system, prompt string, emit func(token string) error) (string, error)
}

// DraftRequest is what to draft and from what.
type DraftRequest struct {
	Kind  Kind
	Title string
	// Source is the material to draft from: the originating request, the
	// parent discovery doc, the product's system doc.
	Source string
	// Current is the draft's existing body, to improve on rather than
	// replace wholesale.
	Current string
	// ProtoPackage is the proto package for KindProto drafts.
	ProtoPackage string
}

// Result is a finished draft.
type Result struct {
	Body string
	// Specs are the domain objects parsed from a design's "Domain objects"
	// table. Always empty for discovery docs.
	Specs []Spec
}

// Spec is one proposed domain object.
type Spec struct {
	Name      string
	Operation string // create | modify | deprecate
	DependsOn []string
}
