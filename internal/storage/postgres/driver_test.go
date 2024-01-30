package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	customerrors "github.com/akashipov/L0project/internal/errors"
	"github.com/akashipov/L0project/internal/storage/order"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSqlWorker_AddData(t *testing.T) {
	type fields struct {
		DB *sql.DB
		TX *sql.Tx
	}
	Start()
	type args struct {
		ctx          context.Context
		dataFilename string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		waitErr bool
	}{
		{
			name: "common_case",
			fields: fields{
				DB: DBWorker.DB,
				TX: nil,
			},
			args: args{
				ctx:          context.Background(),
				dataFilename: "order.json",
			},
			waitErr: false,
		},
		{
			name: "common_again",
			fields: fields{
				DB: DBWorker.DB,
				TX: nil,
			},
			args: args{
				ctx:          context.Background(),
				dataFilename: "order.json",
			},
			waitErr: true,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, tt := range tests {
		d := os.Getenv("PROJECT_DIR")
		f, err := os.OpenFile(
			filepath.Join(d, "cmd/publisher/"+tt.args.dataFilename),
			os.O_RDONLY, 0000,
		)
		require.Equal(t, nil, err)
		data, err := io.ReadAll(f)
		require.Equal(t, nil, err)
		w := &SqlWorker{
			DB: tt.fields.DB,
			TX: tt.fields.TX,
		}
		defer func(c context.Context, d []byte) {
			err := w.DeleteDataByOrderID(c, d)
			if err != nil {
				fmt.Println("Data was not deleted:", err.Error())
			} else {
				fmt.Println("Data were deleted!")
			}
		}(tt.args.ctx, data)
		t.Run(tt.name, func(t *testing.T) {
			err := w.AddData(tt.args.ctx, data)
			if tt.waitErr {
				require.NotEqual(t, nil, err)
				return
			} else {
				require.Equal(t, nil, err)
			}

			var ord order.Order
			err = json.Unmarshal(data, &ord)
			require.Equal(t, nil, err)
			var ordFromDB *order.Order
			ordFromDB, cErr := w.GetOrderByID(ctx, ord.OrderID)
			var emptyCErr *customerrors.CustomError
			require.Equal(t, emptyCErr, cErr)
			assert.Equal(t, ord.DateCreated, ordFromDB.DateCreated)
		})
	}
}
