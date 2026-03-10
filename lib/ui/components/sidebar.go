//go:build ignore

package components

import (
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Sidebar renders the sidebar navigation
type Sidebar struct {
	app.Compo
}

func (s *Sidebar) Render() app.UI {
	return app.Aside().
		Body(
			app.Nav().
				Body(
					app.P().Class("sidebar-label").Text("Navigation"),
					app.Ul().
						Body(
							app.Li().Body(
								app.A().
									Href("/").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/")
									}).
									Text("🏠 Home"),
							),
							app.Li().Body(
								app.A().
									Href("/messages").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/messages")
									}).
									Text("📄 Messages"),
							),
							app.Li().Body(
								app.A().
									Href("/conversations").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/conversations")
									}).
									Text("📄 Conversations"),
							),
							app.Li().Body(
								app.A().
									Href("/resources").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/resources")
									}).
									Text("📄 Resources"),
							),
							app.Li().Body(
								app.A().
									Href("/roles").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/roles")
									}).
									Text("📄 Roles"),
							),
						),
				),
		)
}
