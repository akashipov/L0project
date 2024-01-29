package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/akashipov/L0project/internal/arguments"
	"github.com/akashipov/L0project/internal/handlers"
	"github.com/akashipov/L0project/internal/storage"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func GetLogger() (*zap.SugaredLogger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	log := *logger.Sugar()
	return &log, nil
}

func main() {
	done := make(chan struct{})
	ctx := context.Background()
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigint
		fmt.Printf("\nSignal: %v\n", sig)
		done <- struct{}{}
		w.Done()
	}()
	err := arguments.ParseArgsServer()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	_, err = postgres.NewSqlWorker()
	storage.InitCache(ctx)
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
		postgres.DBWorker.AddData(ctx, m.Data)
	})
	log, err := GetLogger()
	if err != nil {
		fmt.Println("Log creation problem " + err.Error())
		return
	}
	srv := &http.Server{Addr: arguments.HPServer, Handler: handlers.ServerRouter(log)}
	w.Add(1)
	go handlers.RunServer(srv, done, &w)
	w.Wait()
	sc.Close()
	fmt.Println("Subscription was closed!")
}
