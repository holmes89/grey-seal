//go:build ignore

package pages

import (
	"context"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ResourceGetComponent
type ResourceGetNavigation struct {
	ResourceUpdateURL func(uuid string) string
	ResourceListURL   func() string
}

// DefaultResourceGetNavigation returns the default navigation URLs
func DefaultResourceGetNavigation() ResourceGetNavigation {
	return ResourceGetNavigation{
		ResourceUpdateURL: func(uuid string) string { return "/resources/" + uuid + "/update" },
		ResourceListURL:   func() string { return "/resources" },
	}
}

type ResourceGetPage struct {
	app.Compo
	ResourceGetComponent
}

// NewResourceGetPage creates a new ResourceGetPage with pre-set ID.
func NewResourceGetPage(id string) *ResourceGetPage {
	p := &ResourceGetPage{}
	p.ResourceGetComponent.id = id
	p.ResourceGetComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *ResourceGetPage) SetID(id string) {
	p.ResourceGetComponent.id = id
	p.ResourceGetComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *ResourceGetPage) SetIDExtractor(fn IDExtractor) {
	p.ResourceGetComponent.IDExtractor = fn
}

func (p *ResourceGetPage) Render() app.UI {
	if p.ResourceGetComponent.Navigation.ResourceUpdateURL == nil {
		p.ResourceGetComponent.Navigation = DefaultResourceGetNavigation()
	}
	return &components.PageLayout{
		Content: &p.ResourceGetComponent,
	}
}

type ResourceGetComponent struct {
	app.Compo

	id             string
	item           *greysealv1.Resource
	loading        bool
	error          string
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     ResourceGetNavigation
}

func (p *ResourceGetComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ResourceGetComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ResourceGetComponent) loadItem(ctx app.Context) {
	// Reset state
	p.item = nil
	p.error = ""

	// Only extract from URL if IDs weren't set programmatically
	if !p.idsInitialized {
		path := ctx.Page().URL().Path
		if p.IDExtractor != nil {
			p.id, _ = p.IDExtractor(path)
		} else {
			p.id, _ = ExtractPathIDs(path)
		}
	}

	if p.id == "" {
		p.error = "Invalid ID"
		p.Update()
		return
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
				p.item = resp.Data
			}
			p.Update()
		})
	}()
}

func (p *ResourceGetComponent) renderDetails() app.UI {
	if p.item == nil {
		return app.Div()
	}

	return app.Div().
		Body(
			app.Header().
				Body(
					app.H2().Text(p.item.Name),
					app.P().Text("Service: " + p.item.Service),
					app.P().Text("Entity: " + p.item.Entity),
					app.P().Text("Source: " + p.item.Source),
					app.P().Text("Path: " + p.item.Path),
					app.P().Text("CreatedAt: " + p.item.CreatedAt.AsTime().Format(time.RFC3339)),
					app.P().Text("IndexedAt: " + p.item.IndexedAt.AsTime().Format(time.RFC3339)),
				),
			app.Div().
				Body(
					&components.ButtonLink{
						Href: p.Navigation.ResourceUpdateURL(p.id),
						Text: "Edit Resource",
					},
					app.Button().
						Class("button outline danger").
						OnClick(p.onDelete).
						Text("Delete Resource"),
				),
		)
}

func (p *ResourceGetComponent) onDelete(ctx app.Context, e app.Event) {
	go func() {
		if err := api.DeleteResource(context.Background(), p.id); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				p.Update()
			})
			return
		}
		listURL := "/resources"
		if p.Navigation.ResourceListURL != nil {
			listURL = p.Navigation.ResourceListURL()
		}
		ctx.Navigate(listURL)
	}()
}

func (p *ResourceGetComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Resource Details"),
				app.If(p.item != nil,
					p.renderDetails(),
				),
			),
	}

	return content
}
