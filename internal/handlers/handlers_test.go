package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/akashipov/L0project/internal/storage/cache"
	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/go-resty/resty/v2"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func PublishNats(data []byte) error {
	sc, err := nats.Connect(nats.DefaultURL)
	defer sc.Close()
	if err != nil {
		return err
	}
	err = sc.Publish("fun", data)
	if err != nil {
		return err
	}
	return nil
}

func TestGetOrder(t *testing.T) {
	type args struct {
		Url string
		ID  string
	}
	ctx := context.Background()
	postgres.Start(ctx, t)
	cache.InitCache(ctx, postgres.Log)
	srv := httptest.NewServer(ServerRouter(postgres.Log))
	defer srv.Close()
	tests := []struct {
		name         string
		args         args
		expectedJSON string
	}{
		{
			name: "common_case",
			args: args{
				Url: srv.URL + "/order/",
				ID:  "b563feb7b2b84b6t428",
			},
			expectedJSON: filepath.Join("statics", "test", "TestGetOrder_common_case.json"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := postgres.Read(tt.expectedJSON)
			require.Equal(t, nil, err)
			err = postgres.DBWorker.AddData(ctx, []byte(b))
			defer func() {
				postgres.DBWorker.DeleteDataByOrderID(ctx, []byte(b))
				postgres.DBWorker.DeleteOrderHistory(ctx, nil, tt.args.ID)
				fmt.Println("Deleted data successfully")
			}()
			require.Equal(t, nil, err)
			client := resty.New()
			res, err := client.R().Get(tt.args.Url + tt.args.ID)
			if err != nil {
				fmt.Println("Error is:", err.Error())
			}
			require.Equal(t, nil, err)
			assert.Equal(t, http.StatusOK, res.StatusCode())
			var ord1 order.Order
			var ord2 order.Order
			err = json.Unmarshal([]byte(b), &ord1)
			require.Equal(t, nil, err)
			err = json.Unmarshal(res.Body(), &ord2)
			require.Equal(t, nil, err)
			ord2.User.AddressID = 0
			ord1.User.AddressID = 0
			assert.Equal(t, *ord2.User, *ord1.User)
			assert.Equal(t, *ord2.PaymentInfo, *ord1.PaymentInfo)
			assert.Equal(t, true, len(ord2.Items) == len(ord1.Items))
		})
	}
}
