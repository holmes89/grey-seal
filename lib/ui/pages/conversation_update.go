//go:build ignore

package pages

import (
	"context"
	"time"
	"strings"
	"fmt"
	"strconv"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ConversationUpdateComponent
type ConversationUpdateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful update
}

// DefaultConversationUpdateNavigation returns the default navigation
func DefaultConversationUpdateNavigation() ConversationUpdateNavigation {
	return ConversationUpdateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/conversations") },
	}
}

type ConversationUpdatePage struct {
	app.Compo
	ConversationUpdateComponent
}

// NewConversationUpdatePage creates a new ConversationUpdatePage with pre-set ID.
func NewConversationUpdatePage(id string) *ConversationUpdatePage {
	p := &ConversationUpdatePage{}
	p.ConversationUpdateComponent.id = id
	p.ConversationUpdateComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *ConversationUpdatePage) SetID(id string) {
	p.ConversationUpdateComponent.id = id
	p.ConversationUpdateComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *ConversationUpdatePage) SetIDExtractor(fn IDExtractor) {
	p.ConversationUpdateComponent.IDExtractor = fn
}

func (p *ConversationUpdatePage) Render() app.UI {
	if p.ConversationUpdateComponent.Navigation.OnSuccess == nil {
		p.ConversationUpdateComponent.Navigation = DefaultConversationUpdateNavigation()
	}
	return &components.PageLayout{
		Content: &p.ConversationUpdateComponent,
	}
}

type ConversationUpdateComponent struct {
	app.Compo

	id string
	conversation_uuid string
	role string
	content string
	resource_uuids string
	feedback string
	created_at string
	loading        bool
	error          string
	submitting     bool
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     ConversationUpdateNavigation
}

func (p *ConversationUpdateComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationUpdateComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationUpdateComponent) loadItem(ctx app.Context) {
	// Only extract from URL if IDs weren't set programmatically
	if !p.idsInitialized {
		path := ctx.Page().URL().Path
		if p.IDExtractor != nil {
			p.id, _ = p.IDExtractor(path)
		} else {
			p.id, _ = ExtractPathIDs(path)
		}
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
				p.conversation_uuid = resp.Data.ConversationUuid
				p.role = resp.Data.Role
				p.content = resp.Data.Content
				p.resource_uuids = strings.Join(resp.Data.ResourceUuids, ", ")
				p.feedback = fmt.Sprint(resp.Data.Feedback)
				p.created_at = resp.Data.CreatedAt.AsTime().Format(time.RFC3339)
			}
			p.Update()
		})
	}()
}

func (p *ConversationUpdateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Update Conversation"),
			app.If(p.loading,
				app.Div().Class("loading").Text("Loading..."),
			),
			app.If(p.error != "",
				app.Div().Class("error").Text(p.error),
			),
			app.If(!p.loading,
				app.Form().
					OnSubmit(p.onSubmit).
					Body(
						&components.FormField{
							Label: "ConversationUuid",
							ID:    "conversation_uuid",
							Input: app.Input().
							Type("text").
							ID("conversation_uuid").
							Name("conversation_uuid").
							Value(p.conversation_uuid).
							OnChange(p.onConversationUuidChange).
							Required(true),
						},
						&components.FormField{
							Label: "Role",
							ID:    "role",
							Input: app.Input().
							Type("text").
							ID("role").
							Name("role").
							Value(p.role).
							OnChange(p.onRoleChange).
							Required(true),
						},
						&components.FormField{
							Label: "Content",
							ID:    "content",
							Input: app.Input().
							Type("text").
							ID("content").
							Name("content").
							Value(p.content).
							OnChange(p.onContentChange).
							Required(true),
						},
						&components.FormField{
							Label: "ResourceUuids",
							ID:    "resource_uuids",
							Input: app.Input().
							Type("text").
							ID("resource_uuids").
							Name("resource_uuids").
							Value(p.resource_uuids).
							OnChange(p.onResourceUuidsChange).
							Required(true),
						},
						&components.FormField{
							Label: "Feedback",
							ID:    "feedback",
							Input: app.Input().
							Type("number").
							ID("feedback").
							Name("feedback").
							Value(p.feedback).
							OnChange(p.onFeedbackChange).
							Required(true),
						},
						&components.FormField{
							Label: "CreatedAt",
							ID:    "created_at",
							Input: app.Input().
							Type("text").
							ID("created_at").
							Name("created_at").
							Value(p.created_at).
							OnChange(p.onCreatedAtChange).
							Required(true),
						},
						app.Div().
							Class("button-group").
							Body(
								app.Button().
									Type("submit").
									Class("button primary").
									Disabled(p.submitting).
									Body(
										app.If(p.submitting,
											app.Text("Submitting..."),
										).Else(
											app.Text("Submit"),
										),
									),
							),
					),
			),
		)

	return content
}

func (p *ConversationUpdateComponent) onConversationUuidChange(ctx app.Context, e app.Event) {
	p.conversation_uuid = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onRoleChange(ctx app.Context, e app.Event) {
	p.role = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onContentChange(ctx app.Context, e app.Event) {
	p.content = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onResourceUuidsChange(ctx app.Context, e app.Event) {
	p.resource_uuids = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onFeedbackChange(ctx app.Context, e app.Event) {
	p.feedback = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.UpdateConversationRequest{
			Data: &greysealv1.Conversation{
				Uuid: p.id,
				ConversationUuid: p.conversation_uuid,
				Role: p.role,
				Content: p.content,
				ResourceUuids: strings.Split(p.resource_uuids, ","),
				Feedback: func() int32 { v, _ := strconv.Atoi(p.feedback); return int32(v) }(),
			},
		}

		_, err := api.UpdateConversation(context.Background(), p.id, req)

		ctx.Dispatch(func(ctx app.Context) {
			p.submitting = false
			if err != nil {
				p.error = err.Error()
				p.Update()
			} else {
				p.Navigation.OnSuccess(ctx)
			}
		})
	}()
}
