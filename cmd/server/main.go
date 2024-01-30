package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/akashipov/L0project/internal/arguments"
	"github.com/akashipov/L0project/internal/pkg/middleware/logger"
	"github.com/akashipov/L0project/internal/server"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/nats-io/nats.go"
)

func SignalWorker(done chan struct{}, w *sync.WaitGroup) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigint
	fmt.Printf("\nSignal: %v\n", sig)
	done <- struct{}{}
	w.Done()
}

func main() {
	done := make(chan struct{})
	ctx := context.Background()
	var w sync.WaitGroup
	w.Add(1)
	go SignalWorker(done, &w)
	err := arguments.ParseArgsServer()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Load cache from psql db to local memory
	_, err = postgres.NewSqlWorker()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = postgres.DBWorker.CreateDefaultTables()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	sc, err := nats.Connect(arguments.NatsURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	sc.Subscribe("foo", func(m *nats.Msg) {
		fmt.Printf("Received a message\n")
		err := postgres.DBWorker.AddData(ctx, m.Data)
		if err != nil {
			fmt.Println("Some error with adding data: " + err.Error())
		}
	})

	log, err := logger.GetLogger()
	if err != nil {
		fmt.Println("Log creation problem " + err.Error())
		return
	}
	srv, err := server.NewServer(ctx, *log)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	w.Add(1)
	go srv.RunServer(done, &w)
	w.Wait()
	sc.Close()
	fmt.Println("Subscription was closed!")
}
