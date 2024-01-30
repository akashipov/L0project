package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/akashipov/L0project/internal/arguments"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.uber.org/zap"
)

var LRUCache *expirable.LRU[string, []byte]

func InitCache(ctx context.Context, log *zap.SugaredLogger) {
	LRUCache = expirable.NewLRU[string, []byte](arguments.CacheSize, nil, time.Second*time.Duration(arguments.CacheTimeLimitSecs))
	ids, err := postgres.DBWorker.GetHistoryInterval(ctx)
	if err != nil {
		log.Infof("Problem with initialization of cache from Psql db:", err.Error())
	}
	for _, id := range ids {
		ord, cErr := postgres.DBWorker.GetDataByID(ctx, id)
		if err != nil {
			err = errors.Join(cErr, err)
			continue
		}
		data, err := json.MarshalIndent(ord, "", "    ")
		if err != nil {
			err = errors.Join(cErr, err)
			continue
		}
		LRUCache.Add(id, data)
	}
	if err != nil {
		log.Infof("Problem with some ids:", err.Error())
	}
	log.Infof("LRU cache created!")
	// fmt.Println("Values:", LRUCache.Values())
}
