//go:build ignore

package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

// Navigation functions for ConversationListComponent
type ConversationListNavigation struct {
	ConversationDetailURL func(uuid string) string
	ConversationUpdateURL func(uuid string) string
	ConversationCreateURL func() string
}

// DefaultConversationListNavigation returns the default navigation URLs
func DefaultConversationListNavigation() ConversationListNavigation {
	return ConversationListNavigation{
		ConversationDetailURL: func(uuid string) string { return "/conversations/" + uuid },
		ConversationUpdateURL: func(uuid string) string { return "/conversations/" + uuid + "/update" },
		ConversationCreateURL: func() string { return "/conversations/create" },
	}
}

type ConversationListPage struct {
	app.Compo
	ConversationListComponent
}

func (p *ConversationListPage) Render() app.UI {
	if p.ConversationListComponent.Navigation.ConversationDetailURL == nil {
		p.ConversationListComponent.Navigation = DefaultConversationListNavigation()
	}
	return &components.PageLayout{
		Content: &p.ConversationListComponent,
	}
}

type ConversationListComponent struct {
	app.Compo

	items      []*greysealv1.Conversation
	loading    bool
	error      string
	Navigation ConversationListNavigation
}

func (p *ConversationListComponent) OnMount(ctx app.Context) {
	p.loading = true
	p.Update()

	go func() {
		resp, err := api.ListConversations(context.Background(), 10)
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

func (p *ConversationListComponent) Render() app.UI {
	content := &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Conversations"),
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
											Href(p.Navigation.ConversationDetailURL(item.Uuid)).
											Text(item.ConversationUuid),
									),
									app.Td().Body(
										&components.ButtonLink{
											Href:    p.Navigation.ConversationUpdateURL(item.Uuid),
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
						Href: p.Navigation.ConversationCreateURL(),
						Text: "Create Conversation",
					},
				),
		),
}

return content
}

func (p *ConversationListComponent) deleteItem(ctx app.Context, uuid string) {
	go func() {
		if err := api.DeleteConversation(context.Background(), uuid); err != nil {
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
