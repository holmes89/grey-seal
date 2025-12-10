package cmd

import (
	"context"
	"fmt"

	"github.com/holmes89/grey-seal/cmd/form"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/spf13/cobra"
)

// listresourcesCmd represents the listresources command
var listResourcesCmd = &cobra.Command{
	Use:   "resource",
	Short: "list a resource",
	RunE:  app.Listresources,
}

func (app *App) Listresources(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)
	defer app.Close()
	count := int32(10)
	req := &services.ListResourcesRequest{
		Count: &count,
	}

	res, err := client.ListResources(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

// getresourceCmd represents the getresource command
var getResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "get a resource",
	RunE:  app.GetResource,
	Args:  cobra.ExactArgs(1),
}

func (app *App) GetResource(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)
	defer app.Close()

	req := &services.GetResourceRequest{
		Uuid: args[0],
	}

	res, err := client.GetResource(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

// createresourceCmd represents the createResource command
var createResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "create a resource",
	RunE:  app.Createresource,
}

func (app *App) Createresource(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)

	defer app.Close()

	f := form.Form[greysealv1.Resource]{}
	resource, err := f.Parse()
	if err != nil {
		return err
	}
	resource.Source = greysealv1.Source_SOURCE_WEBSITE

	req := &services.CreateResourceRequest{
		Data: &resource,
	}

	res, err := client.CreateResource(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

func init() {

	listCmd.AddCommand(listResourcesCmd)

	getCmd.AddCommand(getResourceCmd)

	createCmd.AddCommand(createResourceCmd)

}
