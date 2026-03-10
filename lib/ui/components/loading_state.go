//go:build ignore

package components

import (
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// LoadingState handles loading, error, and content display states
type LoadingState struct {
	app.Compo
	Loading bool
	Error   string
	Content app.UI
}

func (l *LoadingState) Render() app.UI {
	return app.Div().
		Body(
			app.If(l.Loading,
				app.Div().Class("loading").Text("Loading..."),
			),
			app.If(l.Error != "",
				app.Div().Class("error").Text(l.Error),
			),
			app.If(!l.Loading && l.Error == "",
				l.Content,
			),
		)
}
