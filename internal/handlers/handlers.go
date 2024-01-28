package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	customerrors "github.com/akashipov/L0project/internal/errors"
	"github.com/akashipov/L0project/internal/pkg/middleware/compress"
	"github.com/akashipov/L0project/internal/pkg/middleware/logger"
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
	ctx := context.Background()
	err := postgres.DBWorker.CreateTx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't create TX"))
		return
	}
	fmt.Printf("ID '%s' is processing...\n", id)
	ord, cErr := postgres.DBWorker.GetOrderByID(ctx, id)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	usr, cErr := postgres.DBWorker.GetUserByPhone(ctx, ord.User.Phonenumber)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	itms, cErr := postgres.DBWorker.GetItemsByOrderID(ctx, ord.OrderID)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	payInfo, cErr := postgres.DBWorker.GetPaymentByID(ctx, ord.PaymentInfo.TransactionID)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	addr, cErr := postgres.DBWorker.GetAddressByID(ctx, usr.AddressID)
	if cErr != nil {
		cErr.ReportError(w)
		return
	}
	err = postgres.DBWorker.TX.Commit()
	if err != nil {
		cErr = &customerrors.CustomError{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		}
		return
	}
	postgres.DBWorker.TX = nil
	usr.Address = *addr
	ord.User = usr
	ord.PaymentInfo = payInfo
	ord.Items = itms
	data, err := json.MarshalIndent(ord, "", "    ")
	if err != nil {
		cErr = &customerrors.CustomError{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		}
		cErr.ReportError(w)
		return
	}
	w.Write([]byte(data))
}
