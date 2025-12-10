package cmd

import (
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	conn *grpc.ClientConn
}

func NewApp() *App {
	conn, err := grpc.NewClient("localhost:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return &App{
		conn: conn,
	}
}

func (app *App) Close() {
	_ = app.conn.Close()
}

var app = NewApp()
