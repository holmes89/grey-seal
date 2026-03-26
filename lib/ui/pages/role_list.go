package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type RoleListNavigation struct {
	RoleDetailURL func(uuid string) string
	RoleUpdateURL func(uuid string) string
	RoleCreateURL func() string
}

func DefaultRoleListNavigation() RoleListNavigation {
	return RoleListNavigation{
		RoleDetailURL: func(uuid string) string { return "/roles/" + uuid },
		RoleUpdateURL: func(uuid string) string { return "/roles/" + uuid + "/update" },
		RoleCreateURL: func() string { return "/roles/create" },
	}
}

type RoleListPage struct {
	app.Compo
	RoleListComponent
}

func (p *RoleListPage) Render() app.UI {
	if p.Navigation.RoleDetailURL == nil {
		p.Navigation = DefaultRoleListNavigation()
	}
	return &components.PageLayout{Content: &p.RoleListComponent}
}

type RoleListComponent struct {
	app.Compo

	RoleSvc    api.RoleService
	items      []*greysealv1.Role
	loading    bool
	error      string
	Navigation RoleListNavigation
}

func (p *RoleListComponent) loadData(ctx context.Context) ([]*greysealv1.Role, error) {
	resp, err := p.RoleSvc.ListRoles(ctx, int32(10))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *RoleListComponent) OnMount(ctx app.Context) {
	p.loading = true
	go func() {
		items, err := p.loadData(context.Background())
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.items = items
			}
			ctx.Update()
		})
	}()
}

func (p *RoleListComponent) Render() app.UI {
	return &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().Body(
			app.H1().Text("Roles"),
			app.Table().Body(
				app.THead().Body(
					app.Tr().Body(
						app.Th().Text("Name"),
						app.Th().Text("Actions"),
					),
				),
				app.TBody().Body(
					app.Range(p.items).Slice(func(i int) app.UI {
						item := p.items[i]
						return app.Tr().Body(
							app.Td().Body(
								app.A().Href(p.Navigation.RoleDetailURL(item.Uuid)).Text(item.Name),
							),
							app.Td().Body(
								&components.ButtonLink{
									Href:    p.Navigation.RoleUpdateURL(item.Uuid),
									Text:    "Edit",
									Variant: "outline",
								},
								app.Button().Class("button outline danger").
									OnClick(func(ctx app.Context, e app.Event) {
										p.deleteItem(ctx, item.Uuid)
									}).Text("Delete"),
							),
						)
					}),
				),
			),
			app.Div().Body(
				&components.ButtonLink{Href: p.Navigation.RoleCreateURL(), Text: "Create Role"},
			),
		),
	}
}

func (p *RoleListComponent) deleteItem(ctx app.Context, uuid string) {
	go func() {
		if err := p.RoleSvc.DeleteRole(context.Background(), uuid); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				ctx.Update()
			})
			return
		}
		ctx.Dispatch(func(ctx app.Context) {
			for i, item := range p.items {
				if item.Uuid == uuid {
					p.items = append(p.items[:i], p.items[i+1:]...)
					break
				}
			}
			ctx.Update()
		})
	}()
}
