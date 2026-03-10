//go:build ignore

package components

import (
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// PageLayout provides the standard page structure with header, sidebar, and main content area
type PageLayout struct {
	app.Compo
	Content app.UI
}

func (p *PageLayout) Render() app.UI {
	return app.Div().
		Body(
			&Header{},
			app.Main().
				Class("container").
				Body(
					app.Div().
						Class("grid").
						Body(
							&Sidebar{},
							app.Article().
								Body(
									p.Content,
								),
						),
				),
		)
}
