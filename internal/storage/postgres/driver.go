package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/akashipov/L0project/internal/arguments"
	customerrors "github.com/akashipov/L0project/internal/errors"
	"github.com/akashipov/L0project/internal/pkg/middleware/logger"
	"github.com/akashipov/L0project/internal/storage/item"
	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/akashipov/L0project/internal/storage/payment"
	"github.com/akashipov/L0project/internal/storage/user"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var o *sync.Once
var Log *zap.SugaredLogger

func Start(ctx context.Context, t *testing.T) {
	if o == nil {
		o = &sync.Once{}
	}
	o.Do(func() {
		arguments.ParseArgsServer()
		arguments.HPServer = "0.0.0.0:8000"
		arguments.PostgresPWD = "620631"
		var err error
		Log, err = logger.GetLogger()
		require.Equal(t, nil, err)
		_, err = NewSqlWorker()
		require.Equal(t, nil, err)
	})
}

type SqlWorker struct {
	DB *sql.DB
}

var DBWorker SqlWorker

func NewSqlWorker() (*SqlWorker, error) {
	DB, err := InitDB()
	if err != nil {
		return nil, fmt.Errorf("Problem with init DB -> %w", err)
	}
	DBWorker = SqlWorker{DB: DB}
	return &DBWorker, nil
}

func (w *SqlWorker) AddOrder(ctx context.Context, tx *sql.Tx, ord order.Order) error {
	var err error
	query := "INSERT INTO orders(order_id, track_number, entry, delivery_user, " +
		"transaction_id, locale, internal_signature, customer_id, delivery_service, shardkey," +
		"sm_id, oof_shard, date_created) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)"
	var transID sql.NullString
	transID.Valid = true
	if ord.PaymentInfo == nil {
		transID.Valid = false
	} else {
		transID.String = ord.PaymentInfo.TransactionID
	}
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Phonenumber,
			transID, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Phonenumber,
			transID, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)

	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add Order query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddOrderHistory(ctx context.Context, tx *sql.Tx, order_id string, t int64) error {
	var err error
	query := "INSERT INTO history(order_id, triggered_at) VALUES($1, TO_TIMESTAMP($2)) ON CONFLICT (order_id) DO UPDATE SET triggered_at = TO_TIMESTAMP($2)"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, order_id, t,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, order_id, t,
		)

	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add History Order query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) DeleteOrderHistory(ctx context.Context, tx *sql.Tx, order_id string) error {
	var err error
	query := "DELETE FROM history WHERE order_id = $1"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, order_id,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, order_id,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add History Order query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddUser(ctx context.Context, tx *sql.Tx, user user.User) error {
	var err error
	query := "INSERT INTO users(phonenumber, name, email, address_id) VALUES($1, $2, $3, $4) ON CONFLICT (phonenumber) DO UPDATE SET phonenumber = $1, name = $2, email = $3, address_id=$4"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Email, user.AddressID,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Email, user.AddressID,
		)

	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add User query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddAddress(ctx context.Context, tx *sql.Tx, add *user.Address) (int64, error) {
	filename := "add_address.sql"
	path := filepath.Join(
		"statics",
		"queries", filename,
	)
	query, err := Read(path)
	if err != nil {
		return -1, fmt.Errorf("Problem with reading query '%s': %w", filename, err)
	}
	var r *sql.Row
	if tx == nil {
		r = w.DB.QueryRowContext(
			ctx, query, add.Zipcode, add.City, add.Address, add.Region,
		)
	} else {
		r = tx.QueryRowContext(
			ctx, query, add.Zipcode, add.City, add.Address, add.Region,
		)
	}
	var i int64
	err = r.Scan(&i)
	if err != nil {
		errRoll := tx.Rollback()
		return -1, fmt.Errorf("Problem with getting id of insert row: %w", errors.Join(err, errRoll))
	}
	return i, nil
}

func (w *SqlWorker) AddItems(ctx context.Context, tx *sql.Tx, items []item.Item) error {
	for _, item := range items {
		err := w.AddItem(ctx, tx, &item)
		if err != nil {
			return fmt.Errorf("Problem with execution of Add Item!S query: %w", err)
		}
	}
	return nil
}

func (w *SqlWorker) AddData(ctx context.Context, data []byte) error {
	var ord order.Order
	tx, err := w.CreateTx()
	if err != nil {
		tx = nil
		return err
	}
	err = json.Unmarshal(data, &ord)
	if err != nil {
		return err
	}
	addressID, err := w.AddAddress(ctx, tx, &ord.User.Address)
	if err != nil {
		return err
	}
	ord.User.AddressID = addressID
	err = w.AddUser(ctx, tx, *ord.User)
	if err != nil {
		return err
	}
	err = w.AddPaymentInfo(ctx, tx, ord.PaymentInfo)
	if err != nil {
		return err
	}
	err = w.AddOrder(ctx, tx, ord)
	if err != nil {
		return err
	}
	for idx := range ord.Items {
		ord.Items[idx].OrderID = ord.OrderID
	}
	err = w.AddItems(ctx, tx, ord.Items)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	tx = nil
	fmt.Printf("Order with '%s' was added successfully\n", ord.OrderID)
	return nil
}

func (w *SqlWorker) DeleteDataByOrderID(ctx context.Context, data []byte) error {
	var ord order.Order
	tx, err := w.CreateTx()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &ord)
	if err != nil {
		return err
	}
	err = w.DeleteItemsByOrderID(ctx, tx, ord.OrderID)
	if err != nil {
		return err
	}
	err = w.DeleteOrderByID(ctx, tx, ord.OrderID)
	if err != nil {
		return err
	}
	err = w.DeletePaymentByID(ctx, tx, ord.PaymentInfo.TransactionID)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (w *SqlWorker) DeleteOrderByID(ctx context.Context, tx *sql.Tx, orderID string) error {
	var err error
	query := "DELETE FROM orders WHERE order_id = $1"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, orderID,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, orderID,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Delete Order By ID: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) DeletePaymentByID(ctx context.Context, tx *sql.Tx, paymentID string) error {
	var err error
	query := "DELETE FROM payments WHERE transaction_id = $1"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, paymentID,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, paymentID,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Delete Payment By ID: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) DeleteItemsByOrderID(ctx context.Context, tx *sql.Tx, orderID string) error {
	var err error
	query := "DELETE FROM items WHERE order_id = $1"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, orderID,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, orderID,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Delete Items By Order ID: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddItem(ctx context.Context, tx *sql.Tx, item *item.Item) error {
	var err error
	query := "INSERT INTO items(chrt_id, track_number, price, rid, name, sale," +
		"size, total_price, nm_id, brand, order_id) VALUES($1, $2, $3, $4, " +
		"$5, $6, $7, $8, $9, $10, $11)"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query,
			item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand,
			item.OrderID,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query,
			item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand,
			item.OrderID,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add Item query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) CreateTx() (*sql.Tx, error) {
	return w.DB.Begin()
}

func (w *SqlWorker) GetHistoryInterval(ctx context.Context, tx *sql.Tx) ([]string, error) {
	var err error
	query := "SELECT order_id FROM history ORDER BY triggered_at DESC LIMIT $1"
	var rows *sql.Rows
	if tx == nil {
		rows, err = w.DB.QueryContext(
			ctx, query, arguments.CacheSize,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx, query, arguments.CacheSize,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return nil, fmt.Errorf("Problem with execution of Add History Order query: %w", errors.Join(err, rollErr))
	}
	defer rows.Close()
	ids := make([]string, 0, 5)
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			rollErr := tx.Rollback()
			err = fmt.Errorf("Problem with Scan Id history block: %w", errors.Join(rollErr, err))
			break
		}
		ids = append(ids, id)
	}
	err = errors.Join(err, rows.Err())
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (w *SqlWorker) AddPaymentInfo(ctx context.Context, tx *sql.Tx, pay *payment.Payment) error {
	var err error
	query := "INSERT INTO payments(transaction_id, request_id, currency, provider_id, amount, payment_dt," +
		"bank, delivery_cost, goods_total, custom_fee) VALUES($1, $2, $3, $4, $5, TO_TIMESTAMP($6), $7, $8, $9, $10)"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, pay.TransactionID, pay.RequestID, pay.Currency, pay.ProviderID,
			pay.Amount, pay.PaymentDateTime, pay.Bank, pay.DeliveryCost, pay.GoodsTotal,
			pay.CustomFee,
		)
	} else {
		_, err = tx.ExecContext(
			ctx, query, pay.TransactionID, pay.RequestID, pay.Currency, pay.ProviderID,
			pay.Amount, pay.PaymentDateTime, pay.Bank, pay.DeliveryCost, pay.GoodsTotal,
			pay.CustomFee,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		return fmt.Errorf("Problem with execution of Add PaymentInfo query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) GetOrderByID(ctx context.Context, tx *sql.Tx, orderID string) (*order.Order, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM orders WHERE order_id = $1"
	var row *sql.Row
	if tx == nil {
		row = w.DB.QueryRowContext(
			ctx, query, orderID,
		)
	} else {
		row = tx.QueryRowContext(
			ctx, query, orderID,
		)
	}
	ord := order.NewOrder()
	err := row.Scan(
		&ord.OrderID, &ord.TrackNumber, &ord.Entry,
		&ord.User.Phonenumber, &ord.PaymentInfo.TransactionID, &ord.Locale,
		&ord.InternalSignature, &ord.CustomerID, &ord.DeliveryService, &ord.ShardKey,
		&ord.SmID, &ord.OofShard, &ord.DateCreated,
	)
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Order By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &ord, nil
}

func (w *SqlWorker) GetPaymentByID(ctx context.Context, tx *sql.Tx, paymentID string) (*payment.Payment, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM payments WHERE transaction_id = $1"
	var row *sql.Row
	if tx == nil {
		row = w.DB.QueryRowContext(
			ctx, query, paymentID,
		)
	} else {
		row = tx.QueryRowContext(
			ctx, query, paymentID,
		)
	}
	var pay payment.Payment
	var t time.Time
	err := row.Scan(&pay.TransactionID, &pay.RequestID, &pay.Currency, &pay.ProviderID,
		&pay.Amount, &t, &pay.Bank, &pay.DeliveryCost,
		&pay.GoodsTotal, &pay.CustomFee,
	)
	pay.PaymentDateTime = t.Unix()
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Payment By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &pay, nil
}

func (w *SqlWorker) GetUserByPhone(ctx context.Context, tx *sql.Tx, phone string) (*user.User, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM users WHERE phonenumber = $1"
	var row *sql.Row
	if tx == nil {
		row = w.DB.QueryRowContext(
			ctx, query, phone,
		)
	} else {
		row = tx.QueryRowContext(
			ctx, query, phone,
		)
	}
	var usr user.User
	err := row.Scan(&usr.Phonenumber, &usr.Name, &usr.Email, &usr.AddressID)
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get User By Phone scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &usr, nil
}

func (w *SqlWorker) GetAddressByID(ctx context.Context, tx *sql.Tx, id int64) (*user.Address, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT zipcode, city, address, region FROM addresses WHERE id = $1"
	var row *sql.Row
	if tx == nil {
		row = w.DB.QueryRowContext(
			ctx, query, id,
		)
	} else {
		row = tx.QueryRowContext(
			ctx, query, id,
		)
	}
	var usr user.Address
	err := row.Scan(&usr.Zipcode, &usr.City, &usr.Address, &usr.Region)
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Address By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &usr, nil
}

func (w *SqlWorker) GetItemsByOrderID(ctx context.Context, tx *sql.Tx, orderID string) ([]item.Item, *customerrors.CustomError) {
	var err error
	var customErr customerrors.CustomError
	query := "SELECT * FROM items WHERE order_id = $1"
	var rows *sql.Rows
	if tx == nil {
		rows, err = w.DB.QueryContext(
			ctx, query, orderID,
		)
	} else {
		rows, err = tx.QueryContext(
			ctx, query, orderID,
		)
	}
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Items By OrderID query: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	defer rows.Close()
	var itms []item.Item
	for rows.Next() {
		var itm item.Item
		err = rows.Scan(
			&itm.ChrtID, &itm.TrackNumber, &itm.Price, &itm.RID, &itm.Name,
			&itm.Sale, &itm.Size, &itm.TotalPrice, &itm.NmID, &itm.Brand, &itm.OrderID,
		)
		if err != nil {
			rollErr := tx.Rollback()
			customErr.Message = fmt.Errorf("Problem with execution of Get Items By OrderID scan: %w", errors.Join(err, rollErr)).Error()
			customErr.Status = http.StatusInternalServerError
			return nil, &customErr
		}
		itms = append(itms, itm)
	}
	err = rows.Err()
	if err != nil {
		rollErr := tx.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Items By OrderID rows.Err: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}

	return itms, nil
}

func InitDB() (*sql.DB, error) {
	dbname := "l0_data"
	DB, err := sql.Open(
		"postgres",
		fmt.Sprintf("host=localhost port=5432 user=artemkashipov password=%s dbname=%s sslmode=disable", arguments.PostgresPWD, dbname),
	)
	if err != nil {
		return nil, fmt.Errorf("Problem with opening DB - '%s': %w", dbname, err)
	}
	err = DB.Ping()
	if err != nil {
		return nil, fmt.Errorf("Problem with pinging DB - '%s': %w", dbname, err)
	}
	return DB, nil
}

func Read(path string) (string, error) {
	basepath := os.Getenv("PROJECT_DIR")
	filePath := filepath.Join(
		basepath,
		path,
	)
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0000)
	if err != nil {
		return "", fmt.Errorf("Problem with opening '%s' file: %w", filePath, err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Problem with reading '%s' file: %w", filePath, err)
	}
	return string(b), nil
}

func (w *SqlWorker) CreateDefaultTables() error {
	filename := "init.sql"
	path := filepath.Join(
		"statics", "queries", filename,
	)
	query, err := Read(path)
	if err != nil {
		return fmt.Errorf("Problem with reading of '%s' query: %w", filename, err)
	}
	_, err = w.DB.Exec(
		query,
	)
	if err != nil {
		return fmt.Errorf("Problem with execution of '%s' query: %w", filename, err)
	}
	return nil
}

func (w *SqlWorker) GetDataByID(ctx context.Context, id string) (*order.Order, *customerrors.CustomError) {
	tx, err := w.CreateTx()
	if err != nil {
		cusErr := customerrors.CustomError{
			Message: "Couldn't create TX",
			Status:  http.StatusInternalServerError,
		}
		return nil, &cusErr
	}
	fmt.Printf("ID '%s' is processing...\n", id)
	ord, cErr := w.GetOrderByID(ctx, tx, id)
	if cErr != nil {
		return nil, cErr
	}
	usr, cErr := w.GetUserByPhone(ctx, tx, ord.User.Phonenumber)
	if cErr != nil {
		return nil, cErr
	}
	itms, cErr := w.GetItemsByOrderID(ctx, tx, ord.OrderID)
	if cErr != nil {
		return nil, cErr
	}
	payInfo, cErr := w.GetPaymentByID(ctx, tx, ord.PaymentInfo.TransactionID)
	if cErr != nil {
		return nil, cErr
	}
	addr, cErr := w.GetAddressByID(ctx, tx, usr.AddressID)
	if cErr != nil {
		return nil, cErr
	}
	err = tx.Commit()
	if err != nil {
		cErr = &customerrors.CustomError{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		}
		return nil, cErr
	}
	tx = nil
	usr.Address = *addr
	ord.User = usr
	ord.PaymentInfo = payInfo
	ord.Items = itms
	return ord, nil
}
