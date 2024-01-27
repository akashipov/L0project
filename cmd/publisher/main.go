package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/nats-io/nats.go"
)

func Replace(s, suffix string) string {
	var buider strings.Builder
	if len(s) < len(suffix) {
		s += strings.Repeat("0", len(suffix)-len(s))
	}
	buider.WriteString(s[:len(s)-len(suffix)])
	buider.WriteString(suffix)
	return buider.String()
}

func main() {
	sc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	p := flag.String("f", "cmd/publisher/order.json", "Path to example of order in json format")
	flag.Parse()
	f, err := os.OpenFile(*p, os.O_RDONLY, 0000)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	var ord order.Order
	err = json.Unmarshal(data, &ord)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	i := int64(0)
	ord.User.Address.City += " 3_O"
	for {
		b := strconv.FormatInt(i, 10)
		ord.OrderID = Replace(ord.OrderID, b)
		ord.User.Phonenumber = Replace(ord.User.Phonenumber, b)
		ord.User.Email = Replace(ord.User.Email, b)
		ord.TrackNumber = Replace(ord.TrackNumber, b)
		ord.PaymentInfo.TransactionID = Replace(ord.PaymentInfo.TransactionID, b)
		ord.PaymentInfo.RequestID = Replace(ord.PaymentInfo.RequestID, b)
		d, err := json.Marshal(ord)
		if err != nil {
			fmt.Println("Problem with json data: " + err.Error())
			return
		}
		err = sc.Publish("foo", d)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		i += 1
		time.Sleep(time.Second)
	}
}
