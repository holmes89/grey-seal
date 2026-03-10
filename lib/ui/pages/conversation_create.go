//go:build ignore

package pages

import (
	"context"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
	"strconv"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ConversationCreateComponent
type ConversationCreateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful creation
}

// DefaultConversationCreateNavigation returns the default navigation
func DefaultConversationCreateNavigation() ConversationCreateNavigation {
	return ConversationCreateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/conversations") },
	}
}

type ConversationCreatePage struct {
	app.Compo
	ConversationCreateComponent
}

func (p *ConversationCreatePage) Render() app.UI {
	if p.ConversationCreateComponent.Navigation.OnSuccess == nil {
		p.ConversationCreateComponent.Navigation = DefaultConversationCreateNavigation()
	}
	return &components.PageLayout{
		Content: &p.ConversationCreateComponent,
	}
}

type ConversationCreateComponent struct {
	app.Compo

	conversation_uuid string
	role string
	content string
	resource_uuids string
	feedback string
	created_at string
	Navigation  ConversationCreateNavigation
	submitting  bool
	error       string
}

func (p *ConversationCreateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Create Conversation"),
			app.If(p.error != "",
				app.Div().Class("error").Text(p.error),
			),
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
		)

	return content
}

func (p *ConversationCreateComponent) onConversationUuidChange(ctx app.Context, e app.Event) {
	p.conversation_uuid = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onRoleChange(ctx app.Context, e app.Event) {
	p.role = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onContentChange(ctx app.Context, e app.Event) {
	p.content = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onResourceUuidsChange(ctx app.Context, e app.Event) {
	p.resource_uuids = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onFeedbackChange(ctx app.Context, e app.Event) {
	p.feedback = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.CreateConversationRequest{
			Data: &greysealv1.Conversation{
				ConversationUuid: p.conversation_uuid,
				Role: p.role,
				Content: p.content,
				ResourceUuids: strings.Split(p.resource_uuids, ","),
				Feedback: func() int32 { v, _ := strconv.Atoi(p.feedback); return int32(v) }(),
				CreatedAt: timestamppb.Now(),
			},
		}

		_, err := api.CreateConversation(context.Background(), req)

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
