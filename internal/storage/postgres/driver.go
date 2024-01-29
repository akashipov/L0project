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
	"time"

	"github.com/akashipov/L0project/internal/arguments"
	customerrors "github.com/akashipov/L0project/internal/errors"
	"github.com/akashipov/L0project/internal/storage/item"
	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/akashipov/L0project/internal/storage/payment"
	"github.com/akashipov/L0project/internal/storage/user"
	_ "github.com/lib/pq"
)

type SqlWorker struct {
	DB *sql.DB
	TX *sql.Tx
}

func (w *SqlWorker) Rollback() error {
	if w.TX != nil {
		err := w.TX.Rollback()
		if err != nil {
			return err
		}
		w.TX = nil
		return nil
	}
	return errors.New("Rollback tx is already nil")
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

func (w *SqlWorker) AddOrder(ctx context.Context, ord order.Order) error {
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
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Phonenumber,
			transID, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)
	} else {
		_, err = w.TX.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Phonenumber,
			transID, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)

	}
	if err != nil {
		rollErr := w.Rollback()
		return fmt.Errorf("Problem with execution of Add Order query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddOrderHistory(ctx context.Context, order_id string, t int64) error {
	var err error
	query := "INSERT INTO history(order_id, triggered_at) VALUES($1, TO_TIMESTAMP($2)) ON CONFLICT (order_id) DO UPDATE SET triggered_at = TO_TIMESTAMP($2)"
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query, order_id, t,
		)
	} else {
		_, err = w.TX.ExecContext(
			ctx, query, order_id, t,
		)

	}
	if err != nil {
		rollErr := w.Rollback()
		return fmt.Errorf("Problem with execution of Add History Order query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddUser(ctx context.Context, user user.User) error {
	var err error
	query := "INSERT INTO users(phonenumber, name, email, address_id) VALUES($1, $2, $3, $4)"
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Email, user.AddressID,
		)
	} else {
		_, err = w.TX.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Email, user.AddressID,
		)

	}
	if err != nil {
		rollErr := w.Rollback()
		return fmt.Errorf("Problem with execution of Add User query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) AddAddress(ctx context.Context, add *user.Address) (int64, error) {
	filename := "add_address.sql"
	query, err := ReadQuery(filename)
	if err != nil {
		return -1, fmt.Errorf("Problem with reading query '%s': %w", filename, err)
	}
	var r *sql.Row
	if w.TX == nil {
		r = w.DB.QueryRowContext(
			ctx, query, add.Zipcode, add.City, add.Address, add.Region,
		)
	} else {
		r = w.TX.QueryRowContext(
			ctx, query, add.Zipcode, add.City, add.Address, add.Region,
		)
	}
	var i int64
	err = r.Scan(&i)
	if err != nil {
		errRoll := w.Rollback()
		return -1, fmt.Errorf("Problem with getting id of insert row: %w", errors.Join(err, errRoll))
	}
	return i, nil
}

func (w *SqlWorker) AddItems(ctx context.Context, items []item.Item) error {
	for _, item := range items {
		err := w.AddItem(ctx, &item)
		if err != nil {
			return fmt.Errorf("Problem with execution of Add Item!S query: %w", err)
		}
	}
	return nil
}

func (w *SqlWorker) AddData(ctx context.Context, data []byte) {
	var ord order.Order
	err := w.CreateTx()
	if err != nil {
		w.TX = nil
		fmt.Println(err.Error())
		return
	}
	err = json.Unmarshal(data, &ord)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	addressID, err := w.AddAddress(ctx, &ord.User.Address)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	ord.User.AddressID = addressID
	err = w.AddUser(ctx, *ord.User)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = w.AddPaymentInfo(ctx, ord.PaymentInfo)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = w.AddOrder(ctx, ord)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for idx := range ord.Items {
		ord.Items[idx].OrderID = ord.OrderID
	}
	err = w.AddItems(ctx, ord.Items)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = w.TX.Commit()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	w.TX = nil
}

func (w *SqlWorker) AddItem(ctx context.Context, item *item.Item) error {
	var err error
	query := "INSERT INTO items(chrt_id, track_number, price, rid, name, sale," +
		"size, total_price, nm_id, brand, order_id) VALUES($1, $2, $3, $4, " +
		"$5, $6, $7, $8, $9, $10, $11)"
	fmt.Printf("Adding item id: %d to order with ID - %s\n", item.ChrtID, item.OrderID)
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query,
			item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand,
			item.OrderID,
		)
	} else {
		_, err = w.TX.ExecContext(
			ctx, query,
			item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand,
			item.OrderID,
		)
	}
	if err != nil {
		rollErr := w.Rollback()
		return fmt.Errorf("Problem with execution of Add Item query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) CreateTx() error {
	tx, err := w.DB.Begin()
	if err != nil {
		return err
	}
	if w.TX != nil {
		errRoll := w.TX.Rollback()
		if errRoll != nil {
			return errRoll
		}
	}
	w.TX = tx
	return nil
}

func (w *SqlWorker) GetHistoryInterval(ctx context.Context) ([]string, error) {
	var err error
	query := "SELECT order_id FROM history ORDER BY triggered_at DESC LIMIT 5"
	var rows *sql.Rows
	if w.TX == nil {
		rows, err = w.DB.QueryContext(
			ctx, query,
		)
	} else {
		rows, err = w.TX.QueryContext(
			ctx, query,
		)
	}
	if err != nil {
		rollErr := w.Rollback()
		return nil, fmt.Errorf("Problem with execution of Add History Order query: %w", errors.Join(err, rollErr))
	}
	defer rows.Close()
	ids := make([]string, 0, 5)
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			rollErr := w.Rollback()
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

func (w *SqlWorker) AddPaymentInfo(ctx context.Context, pay *payment.Payment) error {
	var err error
	query := "INSERT INTO payments(transaction_id, request_id, currency, provider_id, amount, payment_dt," +
		"bank, delivery_cost, goods_total, custom_fee) VALUES($1, $2, $3, $4, $5, TO_TIMESTAMP($6), $7, $8, $9, $10)"
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query, pay.TransactionID, pay.RequestID, pay.Currency, pay.ProviderID,
			pay.Amount, pay.PaymentDateTime, pay.Bank, pay.DeliveryCost, pay.GoodsTotal,
			pay.CustomFee,
		)
	} else {
		_, err = w.TX.ExecContext(
			ctx, query, pay.TransactionID, pay.RequestID, pay.Currency, pay.ProviderID,
			pay.Amount, pay.PaymentDateTime, pay.Bank, pay.DeliveryCost, pay.GoodsTotal,
			pay.CustomFee,
		)
	}
	if err != nil {
		rollErr := w.Rollback()
		return fmt.Errorf("Problem with execution of Add PaymentInfo query: %w", errors.Join(err, rollErr))
	}
	return nil
}

func (w *SqlWorker) GetOrderByID(ctx context.Context, orderID string) (*order.Order, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM orders WHERE order_id = $1"
	var row *sql.Row
	if w.TX == nil {
		row = w.DB.QueryRowContext(
			ctx, query, orderID,
		)
	} else {
		row = w.TX.QueryRowContext(
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
		rollErr := w.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Order By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &ord, nil
}

func (w *SqlWorker) GetPaymentByID(ctx context.Context, paymentID string) (*payment.Payment, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM payments WHERE transaction_id = $1"
	var row *sql.Row
	if w.TX == nil {
		row = w.DB.QueryRowContext(
			ctx, query, paymentID,
		)
	} else {
		row = w.TX.QueryRowContext(
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
		rollErr := w.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Payment By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &pay, nil
}

func (w *SqlWorker) GetUserByPhone(ctx context.Context, phone string) (*user.User, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT * FROM users WHERE phonenumber = $1"
	var row *sql.Row
	if w.TX == nil {
		row = w.DB.QueryRowContext(
			ctx, query, phone,
		)
	} else {
		row = w.TX.QueryRowContext(
			ctx, query, phone,
		)
	}
	var usr user.User
	err := row.Scan(&usr.Phonenumber, &usr.Name, &usr.Email, &usr.AddressID)
	if err != nil {
		rollErr := w.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get User By Phone scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &usr, nil
}

func (w *SqlWorker) GetAddressByID(ctx context.Context, id int64) (*user.Address, *customerrors.CustomError) {
	var customErr customerrors.CustomError
	query := "SELECT zipcode, city, address, region FROM addresses WHERE id = $1"
	var row *sql.Row
	if w.TX == nil {
		row = w.DB.QueryRowContext(
			ctx, query, id,
		)
	} else {
		row = w.TX.QueryRowContext(
			ctx, query, id,
		)
	}
	var usr user.Address
	err := row.Scan(&usr.Zipcode, &usr.City, &usr.Address, &usr.Region)
	if err != nil {
		rollErr := w.Rollback()
		customErr.Message = fmt.Errorf("Problem with execution of Get Address By ID scan: %w", errors.Join(err, rollErr)).Error()
		customErr.Status = http.StatusInternalServerError
		return nil, &customErr
	}
	return &usr, nil
}

func (w *SqlWorker) GetItemsByOrderID(ctx context.Context, orderID string) ([]item.Item, *customerrors.CustomError) {
	var err error
	var customErr customerrors.CustomError
	query := "SELECT * FROM items WHERE order_id = $1"
	var rows *sql.Rows
	if w.TX == nil {
		rows, err = w.DB.QueryContext(
			ctx, query, orderID,
		)
	} else {
		rows, err = w.TX.QueryContext(
			ctx, query, orderID,
		)
	}
	if err != nil {
		rollErr := w.Rollback()
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
			rollErr := w.Rollback()
			customErr.Message = fmt.Errorf("Problem with execution of Get Items By OrderID scan: %w", errors.Join(err, rollErr)).Error()
			customErr.Status = http.StatusInternalServerError
			return nil, &customErr
		}
		itms = append(itms, itm)
	}
	err = rows.Err()
	if err != nil {
		rollErr := w.Rollback()
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

func ReadQuery(filename string) (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Problem with get pwd: %w", err)
	}
	filePath := filepath.Join(
		d,
		"internal", "storage", "postgres",
		"queries", filename,
	)
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0000)
	if err != nil {
		return "", fmt.Errorf("Problem with opening '%s' file: %w", filename, err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Problem with reading '%s' file: %w", filename, err)
	}
	return string(b), nil
}

func (w *SqlWorker) CreateDefaultTables() error {
	filename := "init.sql"
	query, err := ReadQuery(filename)
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
	err := w.CreateTx()
	if err != nil {
		cusErr := customerrors.CustomError{
			Message: "Couldn't create TX",
			Status:  http.StatusInternalServerError,
		}
		return nil, &cusErr
	}
	fmt.Printf("ID '%s' is processing...\n", id)
	ord, cErr := w.GetOrderByID(ctx, id)
	if cErr != nil {
		return nil, cErr
	}
	usr, cErr := w.GetUserByPhone(ctx, ord.User.Phonenumber)
	if cErr != nil {
		return nil, cErr
	}
	itms, cErr := w.GetItemsByOrderID(ctx, ord.OrderID)
	if cErr != nil {
		return nil, cErr
	}
	payInfo, cErr := w.GetPaymentByID(ctx, ord.PaymentInfo.TransactionID)
	if cErr != nil {
		return nil, cErr
	}
	addr, cErr := w.GetAddressByID(ctx, usr.AddressID)
	if cErr != nil {
		return nil, cErr
	}
	err = w.TX.Commit()
	if err != nil {
		cErr = &customerrors.CustomError{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		}
		return nil, cErr
	}
	w.TX = nil
	usr.Address = *addr
	ord.User = usr
	ord.PaymentInfo = payInfo
	ord.Items = itms
	return ord, nil
}
