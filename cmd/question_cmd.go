package cmd

import (
	"context"
	"fmt"

	"github.com/holmes89/grey-seal/cmd/form"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	services "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/spf13/cobra"
)

// listquestionsCmd represents the listquestions command
var listquestionsCmd = &cobra.Command{
	Use:   "question",
	Short: "list a question",
	RunE:  app.Listquestions,
}

func (app *App) Listquestions(cmd *cobra.Command, args []string) error {
	client := services.NewQuestionServiceClient(app.conn)
	defer app.Close()
	count := int32(10)
	req := &services.ListQuestionsRequest{
		Count: &count,
	}

	res, err := client.ListQuestions(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

// getquestionCmd represents the getquestion command
var getquestionCmd = &cobra.Command{
	Use:   "question",
	Short: "get a question",
	RunE:  app.Getquestion,
	Args:  cobra.ExactArgs(1),
}

func (app *App) Getquestion(cmd *cobra.Command, args []string) error {
	client := services.NewQuestionServiceClient(app.conn)
	defer app.Close()

	req := &services.GetQuestionRequest{
		Uuid: args[0],
	}

	res, err := client.GetQuestion(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

// createquestionCmd represents the createquestion command
var createquestionCmd = &cobra.Command{
	Use:   "question",
	Short: "create a question",
	RunE:  app.Createquestion,
}

func (app *App) Createquestion(cmd *cobra.Command, args []string) error {
	client := services.NewQuestionServiceClient(app.conn)

	defer app.Close()

	f := form.Form[greysealv1.Question]{}
	question, err := f.Parse()
	if err != nil {
		return err
	}
	req := &services.CreateQuestionRequest{
		Data: &question,
	}

	res, err := client.CreateQuestion(context.Background(), req)
	fmt.Println(res) // todo table print
	return err
}

func init() {

	listCmd.AddCommand(listquestionsCmd)

	getCmd.AddCommand(getquestionCmd)

	createCmd.AddCommand(createquestionCmd)

}
