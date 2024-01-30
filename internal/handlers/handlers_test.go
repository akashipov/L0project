package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/akashipov/L0project/internal/storage/cache"
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
	postgres.Start()
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
			client := resty.New()
			res, err := client.R().Get(tt.args.Url + tt.args.ID)
			require.Equal(t, nil, err)
			assert.Equal(t, http.StatusOK, res.StatusCode())
			b, err := postgres.Read(tt.expectedJSON)
			require.Equal(t, nil, err)
			assert.Equal(t, string(b), string(res.Body()))
		})
	}
}
