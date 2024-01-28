package storage

import (
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

var LRUCache *expirable.LRU[string, []byte]

func InitCache() {
	LRUCache = expirable.NewLRU[string, []byte](5, nil, time.Second*5)
	fmt.Println("LRU cache created!")
}
