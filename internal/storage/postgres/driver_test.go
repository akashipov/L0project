package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	}
	ctx := context.Background()
	Start(ctx, t)
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
			},
			args: args{
				ctx:          ctx,
				dataFilename: "order.json",
			},
			waitErr: false,
		},
		{
			name: "common_again",
			fields: fields{
				DB: DBWorker.DB,
			},
			args: args{
				ctx:          ctx,
				dataFilename: "order.json",
			},
			waitErr: true,
		},
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for _, tt := range tests {
		data, err := Read("/statics/test/" + tt.args.dataFilename)
		require.Equal(t, nil, err)
		w := &SqlWorker{
			DB: tt.fields.DB,
		}
		defer func(c context.Context, d []byte) {
			err := w.DeleteDataByOrderID(c, d)
			if err != nil {
				fmt.Println("Data was not deleted:", err.Error())
			} else {
				fmt.Println("Data were deleted!")
			}
		}(tt.args.ctx, []byte(data))
		t.Run(tt.name, func(t *testing.T) {
			err := w.AddData(tt.args.ctx, []byte(data))
			if tt.waitErr {
				require.NotEqual(t, nil, err)
				return
			} else {
				require.Equal(t, nil, err)
			}

			var ord order.Order
			err = json.Unmarshal([]byte(data), &ord)
			require.Equal(t, nil, err)
			var ordFromDB *order.Order
			ordFromDB, cErr := w.GetOrderByID(ctx, nil, ord.OrderID)
			var emptyCErr *customerrors.CustomError
			require.Equal(t, emptyCErr, cErr)
			assert.Equal(t, ord.DateCreated, ordFromDB.DateCreated)
		})
	}
}
