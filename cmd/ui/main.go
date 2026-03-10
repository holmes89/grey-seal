//go:build ignore

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/holmes89/grey-seal/lib/ui/pages"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
)

func main() {
	// UUID regex pattern
	uuid := "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"

	// Configure the app routes
	app.Route("/", &pages.HomePage{})
	app.Route("/messages", &pages.MessageListPage{})
	app.Route("/messages/create", &pages.MessageCreatePage{})
	app.RouteWithRegexp(fmt.Sprintf("^/messages/(%s)$", uuid), &pages.MessageGetPage{})
	app.RouteWithRegexp(fmt.Sprintf("^/messages/(%s)/update$", uuid), &pages.MessageUpdatePage{})
	app.Route("/conversations", &pages.ConversationListPage{})
	app.Route("/conversations/create", &pages.ConversationCreatePage{})
	app.RouteWithRegexp(fmt.Sprintf("^/conversations/(%s)$", uuid), &pages.ConversationGetPage{})
	app.RouteWithRegexp(fmt.Sprintf("^/conversations/(%s)/update$", uuid), &pages.ConversationUpdatePage{})
	app.Route("/resources", &pages.ResourceListPage{})
	app.Route("/resources/create", &pages.ResourceCreatePage{})
	app.RouteWithRegexp(fmt.Sprintf("^/resources/(%s)$", uuid), &pages.ResourceGetPage{})
	app.RouteWithRegexp(fmt.Sprintf("^/resources/(%s)/update$", uuid), &pages.ResourceUpdatePage{})
	app.Route("/roles", &pages.RoleListPage{})
	app.Route("/roles/create", &pages.RoleCreatePage{})
	app.RouteWithRegexp(fmt.Sprintf("^/roles/(%s)$", uuid), &pages.RoleGetPage{})
	app.RouteWithRegexp(fmt.Sprintf("^/roles/(%s)/update$", uuid), &pages.RoleUpdatePage{})

	app.RunWhenOnBrowser()

	// Configure HTTP server
	http.Handle("/", &app.Handler{
		Name:        "Greyseal",
		Description: "Greyseal Management UI",
		RawHeaders: []string{
			"<script>document.documentElement.setAttribute('data-theme','light')</script>",
		},
		Styles: []string{
			"/web/pico-main/css/pico.blue.min.css",
			"/web/app.css",
		},
	})

	log.Println("Starting server on :8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
