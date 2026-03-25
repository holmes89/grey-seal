package components

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// ButtonLink renders a link styled as a button with client-side navigation.
type ButtonLink struct {
	app.Compo
	Href    string
	Text    string
	Variant string // "primary" (default), "outline", "contrast", "contrast outline"
}

func (b *ButtonLink) Render() app.UI {
	variant := b.Variant
	if variant == "" {
		variant = "primary"
	}

	link := app.A().
		Href(b.Href).
		Role("button").
		Text(b.Text).
		OnClick(func(ctx app.Context, e app.Event) {
			e.PreventDefault()
			ctx.Navigate(b.Href)
		})

	if variant != "primary" {
		link = link.Class(variant)
	}

	return link
}
