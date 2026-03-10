//go:build ignore

package cmd

import (
	"context"
	"fmt"

	"github.com/holmes89/grey-seal/cmd/form"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/spf13/cobra"
)

// listResourcesCmd represents the listResources command
var listResourcesCmd = &cobra.Command{
	Use:   "resource",
	Short: "list a resource",
	RunE:  app.ListResources,
}

func (app *App) ListResources(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)
	defer app.Close()
	count := int32(10)
	req := &services.ListResourcesRequest{
		Count: &count,
	}

	res, err := client.ListResources(context.Background(), req)
	if err != nil {
		return err
	}
	for _, item := range res.Data {
		fmt.Println(item.String())
	}
	return nil
}

// getResourceCmd represents the getResource command
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
	if err != nil {
		return err
	}
	fmt.Println(res.Data.String())
	return nil
}

// createResourceCmd represents the createResource command
var createResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "create a resource",
	RunE:  app.CreateResource,
}

func (app *App) CreateResource(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)

	defer app.Close()

	f := form.Form[greysealv1.Resource]{}
	resource, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.CreateResourceRequest{
		Data: &resource,
	}

	res, err := client.CreateResource(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Created Resource: %s\n", res.Data.Uuid)
	fmt.Println(res.Data.String())
	return nil
}

// updateResourceCmd represents the updateResource command
var updateResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "update a resource",
	RunE:  app.UpdateResource,
	Args:  cobra.ExactArgs(1),
}

func (app *App) UpdateResource(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)
	defer app.Close()

	f := form.Form[greysealv1.Resource]{}
	resource, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.UpdateResourceRequest{
		Uuid: args[0],
		Data: &resource,
	}

	res, err := client.UpdateResource(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Updated Resource: %s\n", args[0])
	fmt.Println(res.Data.String())
	return nil
}

// deleteResourceCmd represents the deleteResource command
var deleteResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "delete a resource",
	RunE:  app.DeleteResource,
	Args:  cobra.ExactArgs(1),
}

func (app *App) DeleteResource(cmd *cobra.Command, args []string) error {
	client := services.NewResourceServiceClient(app.conn)
	defer app.Close()

	req := &services.DeleteResourceRequest{
		Uuid: args[0],
	}

	_, err := client.DeleteResource(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted Resource: %s\n", args[0])
	return nil
}

func init() {

	listCmd.AddCommand(listResourcesCmd)

	getCmd.AddCommand(getResourceCmd)

	createCmd.AddCommand(createResourceCmd)

	updateCmd.AddCommand(updateResourceCmd)

	deleteCmd.AddCommand(deleteResourceCmd)

}
