package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/akashipov/L0project/internal/arguments"
	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/nats-io/nats.go"
)

// func othertest() (x int) {
// 	defer func() {
// 		fmt.Println("we are here")
// 		fmt.Println(&x)
// 		x++
// 	}()
// 	x = 1
// 	return
// }

// func test() int {
// 	var x int
// 	defer func() {
// 		fmt.Println("we are here")
// 		fmt.Println(&x)
// 		x++
// 	}()
// 	x = 1
// 	return x
// }

func main() {
	done := make(chan struct{})
	ctx := context.Background()
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigint
		fmt.Printf("\nSignal: %v\n", sig)
		done <- struct{}{}
	}()
	err := arguments.ParseArgsServer()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	dbWorker, err := postgres.NewSqlWorker()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = postgres.CreateDefaultTables(dbWorker.DB)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(arguments.NatsURL)
	sc, err := nats.Connect(arguments.NatsURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	sc.Subscribe("foo", func(m *nats.Msg) {
		var ord order.Order
		fmt.Printf("Received a message\n")
		err := json.Unmarshal(m.Data, &ord)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		addressID, err := dbWorker.AddAddress(ctx, &ord.User.Address)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Printf("Adress ID '%d' was added\n", addressID)

		err = dbWorker.CreateTx()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ord.User.AddressID = addressID
		err = dbWorker.AddUser(ctx, *ord.User)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		err = dbWorker.AddPaymentInfo(ctx, ord.PaymentInfo)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ordNew := ord
		err = dbWorker.AddOrder(ctx, ordNew)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		err = dbWorker.TX.Commit()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		dbWorker.TX = nil
	})
	<-done
	sc.Close()
	fmt.Println("Subscription was closed!")
	fmt.Println("Server is done")
}
