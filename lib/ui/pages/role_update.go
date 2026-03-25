package pages

import (
	"context"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type RoleUpdateNavigation struct {
	OnSuccess func(ctx app.Context)
}

func DefaultRoleUpdateNavigation() RoleUpdateNavigation {
	return RoleUpdateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/roles") },
	}
}

type RoleUpdatePage struct {
	app.Compo
	RoleUpdateComponent
}

func NewRoleUpdatePage(id string) *RoleUpdatePage {
	p := &RoleUpdatePage{}
	p.RoleUpdateComponent.id = id
	p.RoleUpdateComponent.idsInitialized = true
	return p
}

func (p *RoleUpdatePage) SetID(id string) {
	p.RoleUpdateComponent.id = id
	p.RoleUpdateComponent.idsInitialized = true
}

func (p *RoleUpdatePage) SetIDExtractor(fn IDExtractor) {
	p.RoleUpdateComponent.IDExtractor = fn
}

func (p *RoleUpdatePage) Render() app.UI {
	if p.RoleUpdateComponent.Navigation.OnSuccess == nil {
		p.RoleUpdateComponent.Navigation = DefaultRoleUpdateNavigation()
	}
	return &components.PageLayout{Content: &p.RoleUpdateComponent}
}

type RoleUpdateComponent struct {
	app.Compo

	RoleSvc        api.RoleService
	id             string
	name           string
	system_prompt  string
	loading        bool
	error          string
	submitting     bool
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     RoleUpdateNavigation
}

func (p *RoleUpdateComponent) loadData(ctx context.Context, id string) (*greysealv1.Role, error) {
	resp, err := p.RoleSvc.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *RoleUpdateComponent) buildUpdateRequest() *servicesv1.UpdateRoleRequest {
	return &servicesv1.UpdateRoleRequest{
		Uuid: p.id,
		Data: &greysealv1.Role{
			Name:         p.name,
			SystemPrompt: p.system_prompt,
		},
	}
}

func (p *RoleUpdateComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleUpdateComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleUpdateComponent) loadItem(ctx app.Context) {
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
				p.name = item.Name
				p.system_prompt = item.SystemPrompt
				_ = item.CreatedAt.AsTime().Format(time.RFC3339)
			}
			ctx.Update()
		})
	}()
}

func (p *RoleUpdateComponent) Render() app.UI {
	return app.Div().Body(
		app.H1().Text("Update Role"),
		app.If(p.loading, func() app.UI {
			return app.Div().Class("loading").Text("Loading...")
		}),
		app.If(p.error != "", func() app.UI {
			return app.Div().Class("error").Text(p.error)
		}),
		app.If(!p.loading, func() app.UI {
			return app.Form().OnSubmit(p.onSubmit).Body(
				&components.FormField{
					Label: "Name",
					ID:    "name",
					Input: app.Input().Type("text").ID("name").Name("name").
						Value(p.name).OnChange(p.onNameChange).Required(true),
				},
				&components.FormField{
					Label: "SystemPrompt",
					ID:    "system_prompt",
					Input: app.Input().Type("text").ID("system_prompt").Name("system_prompt").
						Value(p.system_prompt).OnChange(p.onSystemPromptChange).Required(true),
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

func (p *RoleUpdateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *RoleUpdateComponent) onSystemPromptChange(ctx app.Context, e app.Event) {
	p.system_prompt = ctx.JSSrc().Get("value").String()
}

func (p *RoleUpdateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	p.submitting = true
	p.error = ""
	go func() {
		_, err := p.RoleSvc.UpdateRole(context.Background(), p.id, p.buildUpdateRequest())
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
