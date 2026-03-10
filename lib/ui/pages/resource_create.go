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

// Navigation functions for ResourceCreateComponent
type ResourceCreateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful creation
}

// DefaultResourceCreateNavigation returns the default navigation
func DefaultResourceCreateNavigation() ResourceCreateNavigation {
	return ResourceCreateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/resources") },
	}
}

type ResourceCreatePage struct {
	app.Compo
	ResourceCreateComponent
}

func (p *ResourceCreatePage) Render() app.UI {
	if p.ResourceCreateComponent.Navigation.OnSuccess == nil {
		p.ResourceCreateComponent.Navigation = DefaultResourceCreateNavigation()
	}
	return &components.PageLayout{
		Content: &p.ResourceCreateComponent,
	}
}

type ResourceCreateComponent struct {
	app.Compo

	name string
	service string
	entity string
	source string
	path string
	created_at string
	indexed_at string
	Navigation  ResourceCreateNavigation
	submitting  bool
	error       string
}

func (p *ResourceCreateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Create Resource"),
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
						Label: "Service",
						ID:    "service",
						Input: app.Input().
							Type("text").
							ID("service").
							Name("service").
							Value(p.service).
							OnChange(p.onServiceChange).
							Required(true),
					},
					&components.FormField{
						Label: "Entity",
						ID:    "entity",
						Input: app.Input().
							Type("text").
							ID("entity").
							Name("entity").
							Value(p.entity).
							OnChange(p.onEntityChange).
							Required(true),
					},
					&components.FormField{
						Label: "Source",
						ID:    "source",
						Input: app.Input().
							Type("text").
							ID("source").
							Name("source").
							Value(p.source).
							OnChange(p.onSourceChange).
							Required(true),
					},
					&components.FormField{
						Label: "Path",
						ID:    "path",
						Input: app.Input().
							Type("text").
							ID("path").
							Name("path").
							Value(p.path).
							OnChange(p.onPathChange).
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
					&components.FormField{
						Label: "IndexedAt",
						ID:    "indexed_at",
						Input: app.Input().
							Type("text").
							ID("indexed_at").
							Name("indexed_at").
							Value(p.indexed_at).
							OnChange(p.onIndexedAtChange).
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

func (p *ResourceCreateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onServiceChange(ctx app.Context, e app.Event) {
	p.service = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onEntityChange(ctx app.Context, e app.Event) {
	p.entity = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onSourceChange(ctx app.Context, e app.Event) {
	p.source = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onPathChange(ctx app.Context, e app.Event) {
	p.path = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onIndexedAtChange(ctx app.Context, e app.Event) {
	p.indexed_at = ctx.JSSrc().Get("value").String()
}

func (p *ResourceCreateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.CreateResourceRequest{
			Data: &greysealv1.Resource{
				Name: p.name,
				Service: p.service,
				Entity: p.entity,
				Source: p.source,
				Path: p.path,
				CreatedAt: timestamppb.Now(),
				IndexedAt: timestamppb.Now(),
			},
		}

		_, err := api.CreateResource(context.Background(), req)

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
