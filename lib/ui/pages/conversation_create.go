package pages

import (
	"context"
	"strings"

	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type ConversationCreateNavigation struct {
	OnSuccess func(ctx app.Context)
}

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
	if p.Navigation.OnSuccess == nil {
		p.Navigation = DefaultConversationCreateNavigation()
	}
	return &components.PageLayout{Content: &p.ConversationCreateComponent}
}

type ConversationCreateComponent struct {
	app.Compo

	ConversationSvc api.ConversationService
	title           string
	role_uuid       string
	resource_uuids  string
	Navigation      ConversationCreateNavigation
	submitting      bool
	error           string
}

func (p *ConversationCreateComponent) buildCreateRequest() *servicesv1.CreateConversationRequest {
	return &servicesv1.CreateConversationRequest{
		Title:         p.title,
		RoleUuid:      p.role_uuid,
		ResourceUuids: strings.Split(p.resource_uuids, ","),
	}
}

func (p *ConversationCreateComponent) Render() app.UI {
	return app.Div().
		Body(
			app.H1().Text("Create Conversation"),
			app.If(p.error != "", func() app.UI {
				return app.Div().Class("error").Text(p.error)
			}),
			app.Form().
				OnSubmit(p.onSubmit).
				Body(
					&components.FormField{
						Label: "Title",
						ID:    "title",
						Input: app.Input().Type("text").ID("title").Name("title").
							Value(p.title).OnChange(p.onTitleChange).Required(true),
					},
					&components.FormField{
						Label: "RoleUuid",
						ID:    "role_uuid",
						Input: app.Input().Type("text").ID("role_uuid").Name("role_uuid").
							Value(p.role_uuid).OnChange(p.onRoleUuidChange),
					},
					&components.FormField{
						Label: "ResourceUuids",
						ID:    "resource_uuids",
						Input: app.Input().Type("text").ID("resource_uuids").Name("resource_uuids").
							Value(p.resource_uuids).OnChange(p.onResourceUuidsChange),
					},
					app.Div().Class("button-group").Body(
						app.Button().Type("submit").Class("button primary").Disabled(p.submitting).
							Body(
								app.If(p.submitting,
									func() app.UI { return app.Text("Submitting...") },
								).Else(
									func() app.UI { return app.Text("Submit") },
								),
							),
					),
				),
		)
}

func (p *ConversationCreateComponent) onTitleChange(ctx app.Context, e app.Event) {
	p.title = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onRoleUuidChange(ctx app.Context, e app.Event) {
	p.role_uuid = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onResourceUuidsChange(ctx app.Context, e app.Event) {
	p.resource_uuids = ctx.JSSrc().Get("value").String()
}

func (p *ConversationCreateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	p.submitting = true
	p.error = ""
	go func() {
		_, err := p.ConversationSvc.CreateConversation(context.Background(), p.buildCreateRequest())
		ctx.Dispatch(func(ctx app.Context) {
			p.submitting = false
			if err != nil {
				p.error = err.Error()
				ctx.Update()
			} else {
				p.Navigation.OnSuccess(ctx)
			}
		})
	}()
}
