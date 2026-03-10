//go:build ignore

package pages

import (
	"context"
	"fmt"
	"time"
	"strings"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ConversationGetComponent
type ConversationGetNavigation struct {
	ConversationUpdateURL func(uuid string) string
	ConversationListURL   func() string
}

// DefaultConversationGetNavigation returns the default navigation URLs
func DefaultConversationGetNavigation() ConversationGetNavigation {
	return ConversationGetNavigation{
		ConversationUpdateURL: func(uuid string) string { return "/conversations/" + uuid + "/update" },
		ConversationListURL:   func() string { return "/conversations" },
	}
}

type ConversationGetPage struct {
	app.Compo
	ConversationGetComponent
}

// NewConversationGetPage creates a new ConversationGetPage with pre-set ID.
func NewConversationGetPage(id string) *ConversationGetPage {
	p := &ConversationGetPage{}
	p.ConversationGetComponent.id = id
	p.ConversationGetComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *ConversationGetPage) SetID(id string) {
	p.ConversationGetComponent.id = id
	p.ConversationGetComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *ConversationGetPage) SetIDExtractor(fn IDExtractor) {
	p.ConversationGetComponent.IDExtractor = fn
}

func (p *ConversationGetPage) Render() app.UI {
	if p.ConversationGetComponent.Navigation.ConversationUpdateURL == nil {
		p.ConversationGetComponent.Navigation = DefaultConversationGetNavigation()
	}
	return &components.PageLayout{
		Content: &p.ConversationGetComponent,
	}
}

type ConversationGetComponent struct {
	app.Compo

	id             string
	item           *greysealv1.Conversation
	loading        bool
	error          string
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     ConversationGetNavigation
}

func (p *ConversationGetComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationGetComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationGetComponent) loadItem(ctx app.Context) {
	// Reset state
	p.item = nil
	p.error = ""

	// Only extract from URL if IDs weren't set programmatically
	if !p.idsInitialized {
		path := ctx.Page().URL().Path
		if p.IDExtractor != nil {
			p.id, _ = p.IDExtractor(path)
		} else {
			p.id, _ = ExtractPathIDs(path)
		}
	}

	if p.id == "" {
		p.error = "Invalid ID"
		p.Update()
		return
	}

	p.loading = true
	p.Update()

	go func() {
		resp, err := api.GetConversation(context.Background(), p.id)
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.item = resp.Data
			}
			p.Update()
		})
	}()
}

func (p *ConversationGetComponent) renderDetails() app.UI {
	if p.item == nil {
		return app.Div()
	}

	return app.Div().
		Body(
			app.Header().
				Body(
					app.H2().Text(p.item.ConversationUuid),
					app.P().Text("ConversationUuid: " + p.item.ConversationUuid),
					app.P().Text("Role: " + p.item.Role),
					app.P().Text("Content: " + p.item.Content),
					app.P().Text("ResourceUuids: " + strings.Join(p.item.ResourceUuids, ", ")),
					app.P().Text("Feedback: " + fmt.Sprint(p.item.Feedback)),
					app.P().Text("CreatedAt: " + p.item.CreatedAt.AsTime().Format(time.RFC3339)),
				),
			app.Div().
				Body(
					&components.ButtonLink{
						Href: p.Navigation.ConversationUpdateURL(p.id),
						Text: "Edit Conversation",
					},
					app.Button().
						Class("button outline danger").
						OnClick(p.onDelete).
						Text("Delete Conversation"),
				),
		)
}

func (p *ConversationGetComponent) onDelete(ctx app.Context, e app.Event) {
	go func() {
		if err := api.DeleteConversation(context.Background(), p.id); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				p.Update()
			})
			return
		}
		listURL := "/conversations"
		if p.Navigation.ConversationListURL != nil {
			listURL = p.Navigation.ConversationListURL()
		}
		ctx.Navigate(listURL)
	}()
}

func (p *ConversationGetComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Conversation Details"),
				app.If(p.item != nil,
					p.renderDetails(),
				),
			),
	}

	return content
}
