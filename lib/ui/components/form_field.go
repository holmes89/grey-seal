package components

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// FormField renders a labeled form field
type FormField struct {
	app.Compo
	Label    string
	ID       string
	Required bool
	Input    app.UI // The actual input/textarea/select element
}

func (f *FormField) Render() app.UI {
	label := app.Label().
		For(f.ID).
		Text(f.Label)

	return app.Div().
		Class("form-group").
		Body(
			label,
			f.Input,
		)
}
