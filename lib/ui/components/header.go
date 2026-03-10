//go:build ignore

package components

import (
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Header renders the sticky page header with brand and entity navigation
type Header struct {
	app.Compo
}

func (h *Header) Render() app.UI {
	return app.Header().
		Class("container-fluid").
		Body(
			app.Nav().
				Body(
					app.Ul().
						Body(
							app.Li().Body(
								app.A().
									Href("/").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/")
									}).
									Body(app.Strong().Text("Greyseal")),
							),
						),
					app.Ul().
						Body(
							app.Li().Body(
								app.A().
									Href("/messages").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/messages")
									}).
									Text("Messages"),
							),
							app.Li().Body(
								app.A().
									Href("/conversations").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/conversations")
									}).
									Text("Conversations"),
							),
							app.Li().Body(
								app.A().
									Href("/resources").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/resources")
									}).
									Text("Resources"),
							),
							app.Li().Body(
								app.A().
									Href("/roles").
									OnClick(func(ctx app.Context, e app.Event) {
										e.PreventDefault()
										ctx.Navigate("/roles")
									}).
									Text("Roles"),
							),
						),
				),
		)
}
