package pages

import (
	"context"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type RoleGetNavigation struct {
	RoleUpdateURL func(uuid string) string
	RoleListURL   func() string
}

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

func NewRoleGetPage(id string) *RoleGetPage {
	p := &RoleGetPage{}
	p.RoleGetComponent.id = id
	p.RoleGetComponent.idsInitialized = true
	return p
}

func (p *RoleGetPage) SetID(id string) {
	p.RoleGetComponent.id = id
	p.RoleGetComponent.idsInitialized = true
}

func (p *RoleGetPage) SetIDExtractor(fn IDExtractor) {
	p.RoleGetComponent.IDExtractor = fn
}

func (p *RoleGetPage) Render() app.UI {
	if p.RoleGetComponent.Navigation.RoleUpdateURL == nil {
		p.RoleGetComponent.Navigation = DefaultRoleGetNavigation()
	}
	return &components.PageLayout{Content: &p.RoleGetComponent}
}

type RoleGetComponent struct {
	app.Compo

	RoleSvc        api.RoleService
	id             string
	item           *greysealv1.Role
	loading        bool
	error          string
	idsInitialized bool
	IDExtractor    IDExtractor
	Navigation     RoleGetNavigation
}

func (p *RoleGetComponent) loadData(ctx context.Context, id string) (*greysealv1.Role, error) {
	resp, err := p.RoleSvc.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *RoleGetComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleGetComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *RoleGetComponent) loadItem(ctx app.Context) {
	p.item = nil
	p.error = ""

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
		ctx.Update()
		return
	}

	p.loading = true
	go func() {
		item, err := p.loadData(context.Background(), p.id)
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.item = item
			}
			ctx.Update()
		})
	}()
}

func (p *RoleGetComponent) renderDetails() app.UI {
	if p.item == nil {
		return app.Div()
	}
	return app.Div().Body(
		app.Header().Body(
			app.H2().Text(p.item.Name),
			app.P().Text("SystemPrompt: "+p.item.SystemPrompt),
			app.P().Text("CreatedAt: "+p.item.CreatedAt.AsTime().Format(time.RFC3339)),
		),
		app.Div().Body(
			&components.ButtonLink{Href: p.Navigation.RoleUpdateURL(p.id), Text: "Edit Role"},
			app.Button().Class("button outline danger").OnClick(p.onDelete).Text("Delete Role"),
		),
	)
}

func (p *RoleGetComponent) onDelete(ctx app.Context, e app.Event) {
	go func() {
		if err := p.RoleSvc.DeleteRole(context.Background(), p.id); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				ctx.Update()
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
	return &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().Body(
			app.H1().Text("Role Details"),
			app.If(p.item != nil, func() app.UI { return p.renderDetails() }),
		),
	}
}
