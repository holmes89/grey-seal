package pages

import (
	"context"
	"strings"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type ConversationUpdateNavigation struct {
	OnSuccess func(ctx app.Context)
}

func DefaultConversationUpdateNavigation() ConversationUpdateNavigation {
	return ConversationUpdateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/conversations") },
	}
}

type ConversationUpdatePage struct {
	app.Compo
	ConversationUpdateComponent
}

func NewConversationUpdatePage(id string) *ConversationUpdatePage {
	p := &ConversationUpdatePage{}
	p.id = id
	p.idsInitialized = true
	return p
}

func (p *ConversationUpdatePage) SetID(id string) {
	p.id = id
	p.idsInitialized = true
}

func (p *ConversationUpdatePage) SetIDExtractor(fn IDExtractor) {
	p.IDExtractor = fn
}

func (p *ConversationUpdatePage) Render() app.UI {
	if p.Navigation.OnSuccess == nil {
		p.Navigation = DefaultConversationUpdateNavigation()
	}
	return &components.PageLayout{Content: &p.ConversationUpdateComponent}
}

type ConversationUpdateComponent struct {
	app.Compo

	ConversationSvc api.ConversationService
	id              string
	title           string
	role_uuid       string
	resource_uuids  string
	loading         bool
	error           string
	submitting      bool
	idsInitialized  bool
	IDExtractor     IDExtractor
	Navigation      ConversationUpdateNavigation
}

func (p *ConversationUpdateComponent) loadData(ctx context.Context, id string) (*greysealv1.Conversation, error) {
	resp, err := p.ConversationSvc.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *ConversationUpdateComponent) buildUpdateRequest() *servicesv1.UpdateConversationRequest {
	title := p.title
	roleUuid := p.role_uuid
	return &servicesv1.UpdateConversationRequest{
		Uuid:          p.id,
		Title:         &title,
		RoleUuid:      &roleUuid,
		ResourceUuids: strings.Split(p.resource_uuids, ","),
	}
}

func (p *ConversationUpdateComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationUpdateComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationUpdateComponent) loadItem(ctx app.Context) {
	if !p.idsInitialized {
		path := ctx.Page().URL().Path
		if p.IDExtractor != nil {
			p.id, _ = p.IDExtractor(path)
		} else {
			p.id, _ = ExtractPathIDs(path)
		}
	}
	p.loading = true
	go func() {
		item, err := p.loadData(context.Background(), p.id)
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.title = item.Title
				p.role_uuid = item.RoleUuid
				p.resource_uuids = strings.Join(item.ResourceUuids, ",")
				_ = item.CreatedAt.AsTime().Format(time.RFC3339)
			}
			ctx.Update()
		})
	}()
}

func (p *ConversationUpdateComponent) Render() app.UI {
	return app.Div().
		Body(
			app.H1().Text("Update Conversation"),
			app.If(p.loading, func() app.UI {
				return app.Div().Class("loading").Text("Loading...")
			}),
			app.If(p.error != "", func() app.UI {
				return app.Div().Class("error").Text(p.error)
			}),
			app.If(!p.loading, func() app.UI {
				return app.Form().OnSubmit(p.onSubmit).
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
					)
			}),
		)
}

func (p *ConversationUpdateComponent) onTitleChange(ctx app.Context, e app.Event) {
	p.title = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onRoleUuidChange(ctx app.Context, e app.Event) {
	p.role_uuid = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onResourceUuidsChange(ctx app.Context, e app.Event) {
	p.resource_uuids = ctx.JSSrc().Get("value").String()
}

func (p *ConversationUpdateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	p.submitting = true
	p.error = ""
	go func() {
		_, err := p.ConversationSvc.UpdateConversation(context.Background(), p.id, p.buildUpdateRequest())
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
