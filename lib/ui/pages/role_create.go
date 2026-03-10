//go:build ignore

package pages

import (
	"context"
	"google.golang.org/protobuf/types/known/timestamppb"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for RoleCreateComponent
type RoleCreateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful creation
}

// DefaultRoleCreateNavigation returns the default navigation
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
	return &components.PageLayout{
		Content: &p.RoleCreateComponent,
	}
}

type RoleCreateComponent struct {
	app.Compo

	name string
	system_prompt string
	created_at string
	Navigation  RoleCreateNavigation
	submitting  bool
	error       string
}

func (p *RoleCreateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Create Role"),
			app.If(p.error != "",
				app.Div().Class("error").Text(p.error),
			),
			app.Form().
				OnSubmit(p.onSubmit).
				Body(
					&components.FormField{
						Label: "Name",
						ID:    "name",
						Input: app.Input().
							Type("text").
							ID("name").
							Name("name").
							Value(p.name).
							OnChange(p.onNameChange).
							Required(true),
					},
					&components.FormField{
						Label: "SystemPrompt",
						ID:    "system_prompt",
						Input: app.Input().
							Type("text").
							ID("system_prompt").
							Name("system_prompt").
							Value(p.system_prompt).
							OnChange(p.onSystemPromptChange).
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

func (p *RoleCreateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *RoleCreateComponent) onSystemPromptChange(ctx app.Context, e app.Event) {
	p.system_prompt = ctx.JSSrc().Get("value").String()
}

func (p *RoleCreateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *RoleCreateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.CreateRoleRequest{
			Data: &greysealv1.Role{
				Name: p.name,
				SystemPrompt: p.system_prompt,
				CreatedAt: timestamppb.Now(),
			},
		}

		_, err := api.CreateRole(context.Background(), req)

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
