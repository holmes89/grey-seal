package pages

import (
	"context"
	"time"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/holmes89/grey-seal/lib/ui/api"
	"github.com/holmes89/grey-seal/lib/ui/components"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type ConversationGetNavigation struct {
	ConversationUpdateURL func(uuid string) string
	ConversationListURL   func() string
}

func DefaultConversationGetNavigation() ConversationGetNavigation {
	return ConversationGetNavigation{
		ConversationUpdateURL: func(uuid string) string { return "/conversations/" + uuid + "/update" },
		ConversationListURL:   func() string { return "/conversations" },
	}
}

type ConversationGetPage struct {
	app.Compo
	ConversationGetComponent
}

func NewConversationGetPage(id string) *ConversationGetPage {
	p := &ConversationGetPage{}
	p.id = id
	p.idsInitialized = true
	return p
}

func (p *ConversationGetPage) SetID(id string) {
	p.id = id
	p.idsInitialized = true
}

func (p *ConversationGetPage) SetIDExtractor(fn IDExtractor) {
	p.IDExtractor = fn
}

func (p *ConversationGetPage) Render() app.UI {
	if p.Navigation.ConversationUpdateURL == nil {
		p.Navigation = DefaultConversationGetNavigation()
	}
	return &components.PageLayout{Content: &p.ConversationGetComponent}
}

type ConversationGetComponent struct {
	app.Compo

	ConversationSvc api.ConversationService
	id              string
	item            *greysealv1.Conversation
	loading         bool
	error           string
	idsInitialized  bool
	IDExtractor     IDExtractor
	Navigation      ConversationGetNavigation
}

func (p *ConversationGetComponent) loadData(ctx context.Context, id string) (*greysealv1.Conversation, error) {
	resp, err := p.ConversationSvc.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (p *ConversationGetComponent) OnMount(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationGetComponent) OnNav(ctx app.Context) {
	p.loadItem(ctx)
}

func (p *ConversationGetComponent) loadItem(ctx app.Context) {
	p.item = nil
	p.error = ""

	if !p.idsInitialized {
		path := ctx.Page().URL().Path
		if p.IDExtractor != nil {
			p.id, _ = p.IDExtractor(path)
		} else {
			p.id, _ = ExtractPathIDs(path)
		}
	}

	if p.id == "" {
		p.error = "Invalid ID"
		ctx.Update()
		return
	}

	p.loading = true
	go func() {
		item, err := p.loadData(context.Background(), p.id)
		ctx.Dispatch(func(ctx app.Context) {
			p.loading = false
			if err != nil {
				p.error = err.Error()
			} else {
				p.item = item
			}
			ctx.Update()
		})
	}()
}

func (p *ConversationGetComponent) renderDetails() app.UI {
	if p.item == nil {
		return app.Div()
	}
	summary := "—"
	if p.item.Summary != "" {
		summary = p.item.Summary
	}
	return app.Div().Body(
		app.Header().Body(
			app.H2().Text(p.item.Title),
		),
		app.Dl().Body(
			app.Dt().Text("Summary"),
			app.Dd().Text(summary),
			app.Dt().Text("Created"),
			app.Dd().Text(p.item.CreatedAt.AsTime().Format(time.RFC3339)),
		),
		app.H3().Text("Messages"),
		p.renderMessages(),
		app.Div().Body(
			&components.ButtonLink{Href: p.Navigation.ConversationUpdateURL(p.id), Text: "Edit Conversation"},
			app.Button().Class("button outline danger").OnClick(p.onDelete).Text("Delete Conversation"),
		),
	)
}

func (p *ConversationGetComponent) renderMessages() app.UI {
	if len(p.item.Messages) == 0 {
		return app.P().Text("No messages.")
	}
	return app.Div().Class("chat-messages").Body(
		app.Range(p.item.Messages).Slice(func(i int) app.UI {
			msg := p.item.Messages[i]
			isAssistant := msg.Role == greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT
			roleClass := "message-user"
			roleLabel := "You"
			if isAssistant {
				roleClass = "message-assistant"
				roleLabel = "Assistant"
			}
			children := []app.UI{
				app.Strong().Text(roleLabel),
				app.P().Text(msg.Content),
			}
			if isAssistant && len(msg.ResourceUuids) > 0 {
				links := make([]app.UI, len(msg.ResourceUuids))
				for j, uuid := range msg.ResourceUuids {
					links[j] = app.Li().Body(
						app.A().Href("/resources/"+uuid).Text(uuid),
					)
				}
				children = append(children,
					app.Details().Body(
						app.Summary().Text("Sources"),
						app.Ul().Body(links...),
					),
				)
			}
			return app.Div().Class("message "+roleClass).Body(children...)
		}),
	)
}

func (p *ConversationGetComponent) onDelete(ctx app.Context, e app.Event) {
	go func() {
		if err := p.ConversationSvc.DeleteConversation(context.Background(), p.id); err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				p.error = err.Error()
				ctx.Update()
			})
			return
		}
		listURL := "/conversations"
		if p.Navigation.ConversationListURL != nil {
			listURL = p.Navigation.ConversationListURL()
		}
		ctx.Navigate(listURL)
	}()
}

func (p *ConversationGetComponent) Render() app.UI {
	return &components.LoadingState{
		Loading: p.loading,
		Error:   p.error,
		Content: app.Div().Body(
			app.H1().Text("Conversation Details"),
			app.If(p.item != nil, func() app.UI { return p.renderDetails() }),
		),
	}
}
