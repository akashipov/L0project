package server

import (
	"context"
	"testing"

	"github.com/akashipov/L0project/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewServer(t *testing.T) {
	type args struct {
		ctx context.Context
		log zap.SugaredLogger
	}
	postgres.Start()
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "1", args: args{context.Background(), *postgres.Log}, wantErr: false},
		{name: "2", args: args{context.Background(), *postgres.Log}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServer(tt.args.ctx, tt.args.log)
			assert.Equal(t, err != nil, tt.wantErr)
			if err != nil {
				assert.Equal(t, "Server has been created already", err.Error())
				return
			}
			assert.NotEqual(t, nil, got.Log)
			assert.NotEqual(t, nil, got.Srv)
		})
	}
}
