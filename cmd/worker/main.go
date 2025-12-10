package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/holmes89/grey-seal/lib/repo/vector/scraper"

	"github.com/holmes89/archaea/kafka"
	"github.com/holmes89/grey-seal/lib/greyseal/question"
	"github.com/holmes89/grey-seal/lib/greyseal/resource"
	"github.com/holmes89/grey-seal/lib/repo"
	"github.com/holmes89/grey-seal/lib/repo/vector"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

type closable interface {
	Close()
}

func main() {

	consumers := make([]closable, 0)
	conn := os.Getenv("DATABASE_URL")
	store, err := repo.NewDatabase(conn, false)
	if err != nil {
		panic(err)
	}
	defer store.Close()
	fmt.Println("created store...")

	fmt.Println("creating embedding service...")
	ollamaLLMEmbedder, err := ollama.New(ollama.WithModel("all-minilm"), ollama.WithServerURL(
		"http://host.docker.internal:11434",
	))
	if err != nil {
		log.Fatal(err)
	}
	ollamaEmbeder, err := embeddings.NewEmbedder(ollamaLLMEmbedder)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("creating LLM service...")
	ollamaLLM, err := ollama.New(ollama.WithModel("deepseek-r1"), ollama.WithServerURL(
		"http://host.docker.internal:11434",
	))
	if err != nil {
		log.Fatal(err)
	}

	resourceVectorDB := vector.NewResourceVectorRepo(
		&repo.ResourceRepo{Conn: store},
		scraper.NewScraper(),
		store,
		ollamaEmbeder)
	if err != nil {
		log.Fatal(err)
	}

	questionsvc := questionService(store, resourceVectorDB, ollamaLLM)
	resourcesvc := resource.NewResourceService(
		resourceVectorDB,
	)

	kconn := os.Getenv("KAFKA_BROKERS")

	consumers = append(consumers, handleQuestion(questionsvc, []string{kconn}))
	consumers = append(consumers, handleResource(resourcesvc, []string{kconn}))

	errs := make(chan error, 2)
	fmt.Println("listening...")
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()
	log.Printf("terminating %s....\n", <-errs)
	for _, c := range consumers {
		fmt.Println("shutting down consumer...")
		c.Close()
	}
}

func handleQuestion(questionsvc question.QuestionService, brokers []string) closable {
	group := "app-1"
	consumer := kafka.NewConsumer(brokers, &group, question.ConvertProto)
	fmt.Println("registering consumer:", "question")
	question.NewQuestionConsumer(consumer, questionsvc)
	return consumer
}

func questionService(store *repo.Conn, resourceVectorDB question.Querier, ollamaLLM llms.Model) question.QuestionService {
	return question.NewQuestionService(
		&repo.QuestionRepo{Conn: store},
		resourceVectorDB,
		ollamaLLM,
	)

}

func handleResource(resourcesvc resource.ResourceService, brokers []string) closable {
	group := "app-1"
	consumer := kafka.NewConsumer(brokers, &group, resource.ConvertProto)
	fmt.Println("registering consumer:", "resource")
	resource.NewResourceConsumer(consumer, resourcesvc)
	return consumer
}

func resourceService(conn *repo.Conn) resource.ResourceService {
	return resource.NewResourceService(
		&repo.ResourceRepo{Conn: conn},
	)

}
