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

// Navigation functions for RoleGetComponent
type RoleGetNavigation struct {
	RoleUpdateURL func(uuid string) string
	RoleListURL   func() string
}

// DefaultRoleGetNavigation returns the default navigation URLs
func DefaultRoleGetNavigation() RoleGetNavigation {
	return RoleGetNavigation{
		RoleUpdateURL: func(uuid string) string { return "/roles/" + uuid + "/update" },
		RoleListURL:   func() string { return "/roles" },
	}
}

type RoleGetPage struct {
	app.Compo
	RoleGetComponent
}

// NewRoleGetPage creates a new RoleGetPage with pre-set ID.
func NewRoleGetPage(id string) *RoleGetPage {
	p := &RoleGetPage{}
	p.RoleGetComponent.id = id
	p.RoleGetComponent.idsInitialized = true
	return p
}

// SetID allows external code to inject the ID.
func (p *RoleGetPage) SetID(id string) {
	p.RoleGetComponent.id = id
	p.RoleGetComponent.idsInitialized = true
}

// SetIDExtractor allows library users to provide their own function for extracting IDs.
func (p *RoleGetPage) SetIDExtractor(fn IDExtractor) {
	p.RoleGetComponent.IDExtractor = fn
}

func (p *RoleGetPage) Render() app.UI {
	if p.RoleGetComponent.Navigation.RoleUpdateURL == nil {
		p.RoleGetComponent.Navigation = DefaultRoleGetNavigation()
	}
	return &components.PageLayout{
		Content: &p.RoleGetComponent,
	}
}

type RoleGetComponent struct {
	app.Compo

	id             string
	item           *greysealv1.Role
	loading        bool
	error          string
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     RoleGetNavigation
}

func (p *RoleGetComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleGetComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleGetComponent) loadItem(ctx app.Context) {
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
		resp, err := api.GetRole(context.Background(), p.id)
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

func (p *RoleGetComponent) renderDetails() app.UI {
	if p.item == nil {
		return app.Div()
	}

	return app.Div().
		Body(
			app.Header().
				Body(
					app.H2().Text(p.item.Name),
					app.P().Text("SystemPrompt: " + p.item.SystemPrompt),
					app.P().Text("CreatedAt: " + p.item.CreatedAt.AsTime().Format(time.RFC3339)),
				),
			app.Div().
				Body(
					&components.ButtonLink{
						Href: p.Navigation.RoleUpdateURL(p.id),
						Text: "Edit Role",
					},
					app.Button().
						Class("button outline danger").
						OnClick(p.onDelete).
						Text("Delete Role"),
				),
		)
}

func (p *RoleGetComponent) onDelete(ctx app.Context, e app.Event) {
	go func() {
		if err := api.DeleteRole(context.Background(), p.id); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				p.Update()
			})
			return
		}
		listURL := "/roles"
		if p.Navigation.RoleListURL != nil {
			listURL = p.Navigation.RoleListURL()
		}
		ctx.Navigate(listURL)
	}()
}

func (p *RoleGetComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Role Details"),
				app.If(p.item != nil,
					p.renderDetails(),
				),
			),
	}

	return content
}
