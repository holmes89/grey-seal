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

// Navigation functions for ResourceUpdateComponent
type ResourceUpdateNavigation struct {
	OnSuccess func(ctx app.Context) // Called after successful update
}

// DefaultResourceUpdateNavigation returns the default navigation
func DefaultResourceUpdateNavigation() ResourceUpdateNavigation {
	return ResourceUpdateNavigation{
		OnSuccess: func(ctx app.Context) { ctx.Navigate("/resources") },
	}
}

type ResourceUpdatePage struct {
	app.Compo
	ResourceUpdateComponent
}

// NewResourceUpdatePage creates a new ResourceUpdatePage with pre-set ID.
func NewResourceUpdatePage(id string) *ResourceUpdatePage {
	p := &ResourceUpdatePage{}
	p.ResourceUpdateComponent.id = id
	p.ResourceUpdateComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *ResourceUpdatePage) SetID(id string) {
	p.ResourceUpdateComponent.id = id
	p.ResourceUpdateComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *ResourceUpdatePage) SetIDExtractor(fn IDExtractor) {
	p.ResourceUpdateComponent.IDExtractor = fn
}

func (p *ResourceUpdatePage) Render() app.UI {
	if p.ResourceUpdateComponent.Navigation.OnSuccess == nil {
		p.ResourceUpdateComponent.Navigation = DefaultResourceUpdateNavigation()
	}
	return &components.PageLayout{
		Content: &p.ResourceUpdateComponent,
	}
}

type ResourceUpdateComponent struct {
	app.Compo

	id string
	name string
	service string
	entity string
	source string
	path string
	created_at string
	indexed_at string
	loading        bool
	error          string
	submitting     bool
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     ResourceUpdateNavigation
}

func (p *ResourceUpdateComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ResourceUpdateComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ResourceUpdateComponent) loadItem(ctx app.Context) {
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
		resp, err := api.GetResource(context.Background(), p.id)

		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.name = resp.Data.Name
				p.service = resp.Data.Service
				p.entity = resp.Data.Entity
				p.source = resp.Data.Source
				p.path = resp.Data.Path
				p.created_at = resp.Data.CreatedAt.AsTime().Format(time.RFC3339)
				p.indexed_at = resp.Data.IndexedAt.AsTime().Format(time.RFC3339)
			}
			p.Update()
		})
	}()
}

func (p *ResourceUpdateComponent) Render() app.UI {
	content := app.Div().
		Body(
			app.H1().Text("Update Resource"),
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
			),
		)

	return content
}

func (p *ResourceUpdateComponent) onNameChange(ctx app.Context, e app.Event) {
	p.name = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onServiceChange(ctx app.Context, e app.Event) {
	p.service = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onEntityChange(ctx app.Context, e app.Event) {
	p.entity = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onSourceChange(ctx app.Context, e app.Event) {
	p.source = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onPathChange(ctx app.Context, e app.Event) {
	p.path = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onCreatedAtChange(ctx app.Context, e app.Event) {
	p.created_at = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onIndexedAtChange(ctx app.Context, e app.Event) {
	p.indexed_at = ctx.JSSrc().Get("value").String()
}

func (p *ResourceUpdateComponent) onSubmit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	p.submitting = true
	p.error = ""
	p.Update()

	go func() {
		req := &servicesv1.UpdateResourceRequest{
			Data: &greysealv1.Resource{
				Uuid: p.id,
				Name: p.name,
				Service: p.service,
				Entity: p.entity,
				Source: p.source,
				Path: p.path,
			},
		}

		_, err := api.UpdateResource(context.Background(), p.id, req)

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
