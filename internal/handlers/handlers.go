package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	customerrors "github.com/akashipov/L0project/internal/errors"
	"github.com/akashipov/L0project/internal/pkg/middleware/compress"
	"github.com/akashipov/L0project/internal/pkg/middleware/logger"
	"github.com/akashipov/L0project/internal/storage/cache"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func RunServer(srv *http.Server, done chan struct{}, w *sync.WaitGroup) {
	w.Add(1)
	go func() {
		srv.ListenAndServe()
		fmt.Println("Server is stopped")
		w.Done()
	}()
	<-done
	fmt.Println("Server is stopping...")
	srv.Close()
	w.Done()
}

func ServerRouter(log *zap.SugaredLogger) http.Handler {
	r := chi.NewRouter()
	r.Get(
		"/order/{id}",
		logger.WithLogging(http.HandlerFunc(GetOrder), log),
	)
	return compress.GzipHandle(r, log)
}

func GetOrder(w http.ResponseWriter, request *http.Request) {
	id := chi.URLParam(request, "id")
	t := time.Now().Unix()
	ctx := context.Background()
	v, ok := cache.LRUCache.Get(id)
	if ok {
		cache.LRUCache.Add(id, v)
		fmt.Println("Returned cached result")
		w.Write(v)
		err := postgres.DBWorker.AddOrderHistory(ctx, id, t)
		if err != nil {
			fmt.Println(err.Error())
		}
		return
	}
	ord, cErr := postgres.DBWorker.GetDataByID(ctx, id)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	data, err := json.MarshalIndent(ord, "", "    ")
	if err != nil {
		cErr = &customerrors.CustomError{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		}
		cErr.ReportError(w)
		return
	}
	cache.LRUCache.Add(id, data)
	w.Write(data)
	err = postgres.DBWorker.AddOrderHistory(ctx, id, t)
	if err != nil {
		fmt.Println(err.Error())
	}
}
