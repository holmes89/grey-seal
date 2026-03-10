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

// listMessagesCmd represents the listMessages command
var listMessagesCmd = &cobra.Command{
	Use:   "message",
	Short: "list a message",
	RunE:  app.ListMessages,
}

func (app *App) ListMessages(cmd *cobra.Command, args []string) error {
	client := services.NewMessageServiceClient(app.conn)
	defer app.Close()
	count := int32(10)
	req := &services.ListMessagesRequest{
		Count: &count,
	}

	res, err := client.ListMessages(context.Background(), req)
	if err != nil {
		return err
	}
	for _, item := range res.Data {
		fmt.Println(item.String())
	}
	return nil
}

// getMessageCmd represents the getMessage command
var getMessageCmd = &cobra.Command{
	Use:   "message",
	Short: "get a message",
	RunE:  app.GetMessage,
	Args:  cobra.ExactArgs(1),
}

func (app *App) GetMessage(cmd *cobra.Command, args []string) error {
	client := services.NewMessageServiceClient(app.conn)
	defer app.Close()

	req := &services.GetMessageRequest{
		Uuid: args[0],
	}

	res, err := client.GetMessage(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Println(res.Data.String())
	return nil
}

// createMessageCmd represents the createMessage command
var createMessageCmd = &cobra.Command{
	Use:   "message",
	Short: "create a message",
	RunE:  app.CreateMessage,
}

func (app *App) CreateMessage(cmd *cobra.Command, args []string) error {
	client := services.NewMessageServiceClient(app.conn)

	defer app.Close()

	f := form.Form[greysealv1.Message]{}
	message, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.CreateMessageRequest{
		Data: &conversation,
	}

	res, err := client.CreateMessage(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Created Message: %s\n", res.Data.Uuid)
	fmt.Println(res.Data.String())
	return nil
}

// updateMessageCmd represents the updateMessage command
var updateMessageCmd = &cobra.Command{
	Use:   "message",
	Short: "update a message",
	RunE:  app.UpdateMessage,
	Args:  cobra.ExactArgs(1),
}

func (app *App) UpdateMessage(cmd *cobra.Command, args []string) error {
	client := services.NewMessageServiceClient(app.conn)
	defer app.Close()

	f := form.Form[greysealv1.Message]{}
	message, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.UpdateMessageRequest{
		Uuid: args[0],
		Data: &conversation,
	}

	res, err := client.UpdateMessage(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Updated Message: %s\n", args[0])
	fmt.Println(res.Data.String())
	return nil
}

// deleteMessageCmd represents the deleteMessage command
var deleteMessageCmd = &cobra.Command{
	Use:   "message",
	Short: "delete a message",
	RunE:  app.DeleteMessage,
	Args:  cobra.ExactArgs(1),
}

func (app *App) DeleteMessage(cmd *cobra.Command, args []string) error {
	client := services.NewMessageServiceClient(app.conn)
	defer app.Close()

	req := &services.DeleteMessageRequest{
		Uuid: args[0],
	}

	_, err := client.DeleteMessage(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted Message: %s\n", args[0])
	return nil
}

func init() {

	listCmd.AddCommand(listMessagesCmd)

	getCmd.AddCommand(getMessageCmd)

	createCmd.AddCommand(createMessageCmd)

	updateCmd.AddCommand(updateMessageCmd)

	deleteCmd.AddCommand(deleteMessageCmd)

}
