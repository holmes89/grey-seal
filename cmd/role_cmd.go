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

// listRolesCmd represents the listRoles command
var listRolesCmd = &cobra.Command{
	Use:   "role",
	Short: "list a role",
	RunE:  app.ListRoles,
}

func (app *App) ListRoles(cmd *cobra.Command, args []string) error {
	client := services.NewRoleServiceClient(app.conn)
	defer app.Close()
	count := int32(10)
	req := &services.ListRolesRequest{
		Count: &count,
	}

	res, err := client.ListRoles(context.Background(), req)
	if err != nil {
		return err
	}
	for _, item := range res.Data {
		fmt.Println(item.String())
	}
	return nil
}

// getRoleCmd represents the getRole command
var getRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "get a role",
	RunE:  app.GetRole,
	Args:  cobra.ExactArgs(1),
}

func (app *App) GetRole(cmd *cobra.Command, args []string) error {
	client := services.NewRoleServiceClient(app.conn)
	defer app.Close()

	req := &services.GetRoleRequest{
		Uuid: args[0],
	}

	res, err := client.GetRole(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Println(res.Data.String())
	return nil
}

// createRoleCmd represents the createRole command
var createRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "create a role",
	RunE:  app.CreateRole,
}

func (app *App) CreateRole(cmd *cobra.Command, args []string) error {
	client := services.NewRoleServiceClient(app.conn)

	defer app.Close()

	f := form.Form[greysealv1.Role]{}
	role, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.CreateRoleRequest{
		Data: &role,
	}

	res, err := client.CreateRole(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Created Role: %s\n", res.Data.Uuid)
	fmt.Println(res.Data.String())
	return nil
}

// updateRoleCmd represents the updateRole command
var updateRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "update a role",
	RunE:  app.UpdateRole,
	Args:  cobra.ExactArgs(1),
}

func (app *App) UpdateRole(cmd *cobra.Command, args []string) error {
	client := services.NewRoleServiceClient(app.conn)
	defer app.Close()

	f := form.Form[greysealv1.Role]{}
	role, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.UpdateRoleRequest{
		Uuid: args[0],
		Data: &role,
	}

	res, err := client.UpdateRole(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Updated Role: %s\n", args[0])
	fmt.Println(res.Data.String())
	return nil
}

// deleteRoleCmd represents the deleteRole command
var deleteRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "delete a role",
	RunE:  app.DeleteRole,
	Args:  cobra.ExactArgs(1),
}

func (app *App) DeleteRole(cmd *cobra.Command, args []string) error {
	client := services.NewRoleServiceClient(app.conn)
	defer app.Close()

	req := &services.DeleteRoleRequest{
		Uuid: args[0],
	}

	_, err := client.DeleteRole(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted Role: %s\n", args[0])
	return nil
}

func init() {

	listCmd.AddCommand(listRolesCmd)

	getCmd.AddCommand(getRoleCmd)

	createCmd.AddCommand(createRoleCmd)

	updateCmd.AddCommand(updateRoleCmd)

	deleteCmd.AddCommand(deleteRoleCmd)

}
