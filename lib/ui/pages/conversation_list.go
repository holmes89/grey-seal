package pages

import (
	"context"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type ConversationListNavigation struct {
	ConversationDetailURL func(uuid string) string
	ConversationUpdateURL func(uuid string) string
	ConversationCreateURL func() string
}

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
	if p.Navigation.ConversationDetailURL == nil {
		p.Navigation = DefaultConversationListNavigation()
	}
	return &components.PageLayout{Content: &p.ConversationListComponent}
}

type ConversationListComponent struct {
	app.Compo

	ConversationSvc api.ConversationService
	items           []*greysealv1.Conversation
	loading         bool
	error           string
	Navigation      ConversationListNavigation
}

func (p *ConversationListComponent) loadData(ctx context.Context) ([]*greysealv1.Conversation, error) {
	resp, err := p.ConversationSvc.ListConversations(ctx, int32(10))
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *ConversationListComponent) OnMount(ctx app.Context) {
	p.loading = true
	go func() {
		items, err := p.loadData(context.Background())
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.items = items
			}
			ctx.Update()
		})
	}()
}

func (p *ConversationListComponent) Render() app.UI {
	return &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().
			Body(
				app.H1().Text("Conversations"),
				app.Table().
					Body(
						app.THead().Body(
							app.Tr().Body(
								app.Th().Text("Title"),
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
											Text(item.Title),
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
				app.Div().Body(
					&components.ButtonLink{
						Href: p.Navigation.ConversationCreateURL(),
						Text: "Create Conversation",
					},
				),
			),
	}
}

func (p *ConversationListComponent) deleteItem(ctx app.Context, uuid string) {
	go func() {
		if err := p.ConversationSvc.DeleteConversation(context.Background(), uuid); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				ctx.Update()
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
			ctx.Update()
		})
	}()
}
