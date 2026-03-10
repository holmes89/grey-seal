//go:build ignore

package pages

import (
	"context"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for RoleUpdateComponent
type RoleUpdateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful update
}

// DefaultRoleUpdateNavigation returns the default navigation
func DefaultRoleUpdateNavigation() RoleUpdateNavigation {
	return RoleUpdateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/roles") },
	}
}

type RoleUpdatePage struct {
	app.Compo
	RoleUpdateComponent
}

// NewRoleUpdatePage creates a new RoleUpdatePage with pre-set ID.
func NewRoleUpdatePage(id string) *RoleUpdatePage {
	p := &RoleUpdatePage{}
	p.RoleUpdateComponent.id = id
	p.RoleUpdateComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *RoleUpdatePage) SetID(id string) {
	p.RoleUpdateComponent.id = id
	p.RoleUpdateComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *RoleUpdatePage) SetIDExtractor(fn IDExtractor) {
	p.RoleUpdateComponent.IDExtractor = fn
}

func (p *RoleUpdatePage) Render() app.UI {
	if p.RoleUpdateComponent.Navigation.OnSuccess == nil {
		p.RoleUpdateComponent.Navigation = DefaultRoleUpdateNavigation()
	}
	return &components.PageLayout{
		Content: &p.RoleUpdateComponent,
	}
}

type RoleUpdateComponent struct {
	app.Compo

	id string
	name string
	system_prompt string
	created_at string
	loading        bool
	error          string
	submitting     bool
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     RoleUpdateNavigation
}

func (p *RoleUpdateComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleUpdateComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleUpdateComponent) loadItem(ctx app.Context) {
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
		resp, err := api.GetRole(context.Background(), p.id)

		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.name = resp.Data.Name
				p.system_prompt = resp.Data.SystemPrompt
				p.created_at = resp.Data.CreatedAt.AsTime().Format(time.RFC3339)
			}
			p.Update()
		})
	}()
}

func (p *RoleUpdateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Update Role"),
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
			),
		)

	return content
}

func (p *RoleUpdateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *RoleUpdateComponent) onSystemPromptChange(ctx app.Context, e app.Event) {
	p.system_prompt = ctx.JSSrc().Get("value").String()
}

func (p *RoleUpdateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *RoleUpdateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.UpdateRoleRequest{
			Data: &greysealv1.Role{
				Uuid: p.id,
				Name: p.name,
				SystemPrompt: p.system_prompt,
			},
		}

		_, err := api.UpdateRole(context.Background(), p.id, req)

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
