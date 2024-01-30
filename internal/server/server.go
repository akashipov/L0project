package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/akashipov/L0project/internal/arguments"
	"github.com/akashipov/L0project/internal/handlers"
	"github.com/akashipov/L0project/internal/storage/cache"
	"go.uber.org/zap"
)

var once *sync.Once

type Server struct {
	Srv *http.Server
	Log *zap.SugaredLogger
}

func NewServer(ctx context.Context, log zap.SugaredLogger) (*Server, error) {
	var srv *http.Server
	if once == nil {
		once = &sync.Once{}
	}
	once.Do(
		func() {
			srv = &http.Server{Addr: arguments.HPServer, Handler: handlers.ServerRouter(&log)}
			cache.InitCache(ctx, &log)
		},
	)
	if srv == nil {
		return nil, errors.New("Server has been created already")
	}
	return &Server{
		Log: &log,
		Srv: srv,
	}, nil
}

func (s *Server) RunServer(done chan struct{}, w *sync.WaitGroup) {
	w.Add(1)
	if s.Srv == nil {
		fmt.Println("Need to init server first")
		return
	}
	go func() {
		s.Srv.ListenAndServe()
		fmt.Println("Server is stopped")
		w.Done()
	}()
	<-done
	fmt.Println("Server is stopping...")
	s.Srv.Close()
	w.Done()
}
