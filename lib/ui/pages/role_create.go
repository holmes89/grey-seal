package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RoleCreateNavigation struct {
	OnSuccess func(ctx app.Context)
}

func DefaultRoleCreateNavigation() RoleCreateNavigation {
	return RoleCreateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/roles") },
	}
}

type RoleCreatePage struct {
	app.Compo
	RoleCreateComponent
}

func (p *RoleCreatePage) Render() app.UI {
	if p.RoleCreateComponent.Navigation.OnSuccess == nil {
		p.RoleCreateComponent.Navigation = DefaultRoleCreateNavigation()
	}
	return &components.PageLayout{Content: &p.RoleCreateComponent}
}

type RoleCreateComponent struct {
	app.Compo

	RoleSvc       api.RoleService
	name          string
	system_prompt string
	Navigation    RoleCreateNavigation
	submitting    bool
	error         string
}

func (p *RoleCreateComponent) buildCreateRequest() *servicesv1.CreateRoleRequest {
	return &servicesv1.CreateRoleRequest{
		Data: &greysealv1.Role{
			Name:         p.name,
			SystemPrompt: p.system_prompt,
			CreatedAt:    timestamppb.Now(),
		},
	}
}

func (p *RoleCreateComponent) Render() app.UI {
	return app.Div().Body(
		app.H1().Text("Create Role"),
		app.If(p.error != "", func() app.UI {
			return app.Div().Class("error").Text(p.error)
		}),
		app.Form().OnSubmit(p.onSubmit).Body(
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
		),
	)
}

func (p *RoleCreateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *RoleCreateComponent) onSystemPromptChange(ctx app.Context, e app.Event) {
	p.system_prompt = ctx.JSSrc().Get("value").String()
}

func (p *RoleCreateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	p.submitting = true
	p.error = ""
	go func() {
		_, err := p.RoleSvc.CreateRole(context.Background(), p.buildCreateRequest())
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
