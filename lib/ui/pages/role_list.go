//go:build ignore

package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for RoleListComponent
type RoleListNavigation struct {
	RoleDetailURL func(uuid string) string
	RoleUpdateURL func(uuid string) string
	RoleCreateURL func() string
}

// DefaultRoleListNavigation returns the default navigation URLs
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
	if p.RoleListComponent.Navigation.RoleDetailURL == nil {
		p.RoleListComponent.Navigation = DefaultRoleListNavigation()
	}
	return &components.PageLayout{
		Content: &p.RoleListComponent,
	}
}

type RoleListComponent struct {
	app.Compo

	items      []*greysealv1.Role
	loading    bool
	error      string
	Navigation RoleListNavigation
}

func (p *RoleListComponent) OnMount(ctx app.Context) {
	p.loading = true
	p.Update()

	go func() {
		resp, err := api.ListRoles(context.Background(), 10)
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.items = resp.Data
			}
			p.Update()
		})
	}()
}

func (p *RoleListComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Roles"),
				app.Table().
					Body(
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
										app.A().
											Href(p.Navigation.RoleDetailURL(item.Uuid)).
											Text(item.Name),
									),
									app.Td().Body(
										&components.ButtonLink{
											Href:    p.Navigation.RoleUpdateURL(item.Uuid),
											Text:    "Edit",
											Variant: "outline",
										},
									app.Button().
										Class("button outline danger").
										OnClick(func(ctx app.Context, e app.Event) {
											p.deleteItem(ctx, item.Uuid)
										}).
										Text("Delete"),
								),
							)
						}),
					),
				),
			app.Div().
				Body(
					&components.ButtonLink{
						Href: p.Navigation.RoleCreateURL(),
						Text: "Create Role",
					},
				),
		),
}

return content
}

func (p *RoleListComponent) deleteItem(ctx app.Context, uuid string) {
	go func() {
		if err := api.DeleteRole(context.Background(), uuid); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				p.Update()
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
			p.Update()
		})
	}()
}
