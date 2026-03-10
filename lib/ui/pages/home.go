//go:build ignore

package pages

import (
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// HomePage is the default home page
type HomePage struct {
	app.Compo
}

func (h *HomePage) Render() app.UI {
	return &components.PageLayout{
		Content: app.Article().
			Body(
				app.H1().Text("Welcome to Greyseal"),
				app.P().Text("Greyseal management UI"),
				app.H2().Text("Getting Started"),
				app.P().Text("Navigate using the sidebar to manage your data."),
			),
	}
}
