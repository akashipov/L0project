package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-resty/resty/v2"
)

func foo() {

	ids := []string{"b563feb7b2b84b6tes0", "b563feb7b2b84b6tes1", "b563feb7b2b84b6tes2",
		"b563feb7b2b84b6tes3", "b563feb7b2b84b6tes4", "b563feb7b2b84b6tes5"}
	for i := 0; i < 10000; i++ {

		go func() {
			cl := resty.New()
			i := rand.Intn(6)
			id := ids[i]
			resp, err := cl.R().Get(
				"http://localhost:8000/order/" + id,
			)
			if err != nil {
				fmt.Printf("Number of gorutine is %d. Error is: %s\n", i, err.Error())
			} else {
				fmt.Printf("Number of gorutine is %d. Status is: %d\n", i, resp.StatusCode())
			}
		}()
		fmt.Println(i)
	}
}

func main() {
	go foo()
	time.Sleep(time.Second * 5)
}
