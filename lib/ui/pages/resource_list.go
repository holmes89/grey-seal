//go:build ignore

package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ResourceListComponent
type ResourceListNavigation struct {
	ResourceDetailURL func(uuid string) string
	ResourceUpdateURL func(uuid string) string
	ResourceCreateURL func() string
}

// DefaultResourceListNavigation returns the default navigation URLs
func DefaultResourceListNavigation() ResourceListNavigation {
	return ResourceListNavigation{
		ResourceDetailURL: func(uuid string) string { return "/resources/" + uuid },
		ResourceUpdateURL: func(uuid string) string { return "/resources/" + uuid + "/update" },
		ResourceCreateURL: func() string { return "/resources/create" },
	}
}

type ResourceListPage struct {
	app.Compo
	ResourceListComponent
}

func (p *ResourceListPage) Render() app.UI {
	if p.ResourceListComponent.Navigation.ResourceDetailURL == nil {
		p.ResourceListComponent.Navigation = DefaultResourceListNavigation()
	}
	return &components.PageLayout{
		Content: &p.ResourceListComponent,
	}
}

type ResourceListComponent struct {
	app.Compo

	items      []*greysealv1.Resource
	loading    bool
	error      string
	Navigation ResourceListNavigation
}

func (p *ResourceListComponent) OnMount(ctx app.Context) {
	p.loading = true
	p.Update()

	go func() {
		resp, err := api.ListResources(context.Background(), 10)
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

func (p *ResourceListComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Resources"),
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
											Href(p.Navigation.ResourceDetailURL(item.Uuid)).
											Text(item.Name),
									),
									app.Td().Body(
										&components.ButtonLink{
											Href:    p.Navigation.ResourceUpdateURL(item.Uuid),
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
						Href: p.Navigation.ResourceCreateURL(),
						Text: "Create Resource",
					},
				),
		),
}

return content
}

func (p *ResourceListComponent) deleteItem(ctx app.Context, uuid string) {
	go func() {
		if err := api.DeleteResource(context.Background(), uuid); err != nil {
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
