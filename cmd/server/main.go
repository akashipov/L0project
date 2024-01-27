package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	defer sc.Close()
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
		ctx := context.Background()
		err = dbWorker.AddUser(nil, ctx, *ord.User)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		ordNew := ord
		ordNew.PaymentInfo = nil
		err = dbWorker.AddOrder(nil, ctx, ordNew)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	})
	for {
		time.Sleep(time.Second * 5)
	}
}
