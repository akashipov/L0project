package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/akashipov/L0project/internal/arguments"
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

func NewSqlWorker() (*SqlWorker, error) {
	DB, err := InitDB()
	if err != nil {
		return nil, fmt.Errorf("Problem with init DB -> %w", err)
	}
	return &SqlWorker{DB: DB}, nil
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

func (w *SqlWorker) AddUser(ctx context.Context, user user.User) error {
	var err error
	query := "INSERT INTO users(phonenumber, name, email, address_id) VALUES($1, $2, $3, $4)"
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Email, user.AddressID,
		)
	} else {
		_, err = w.DB.ExecContext(
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
			return fmt.Errorf("Problem with execution of Add Item query: %w", err)
		}
	}
	return nil
}

func (w *SqlWorker) AddItem(ctx context.Context, item *item.Item) error {
	var err error
	query := "INSERT INTO items(chrt_id, track_number, price, rid, name, sale," +
		"size, total_price, nm_id, brand, order_id) VALUES($1, $2, $3, $4, " +
		"$5, $6, $7, $8, $9, $10, $11)"
	if w.TX == nil {
		_, err = w.DB.ExecContext(
			ctx, query,
			item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand,
			item.OrderID,
		)
	} else {
		_, err = w.DB.ExecContext(
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

func InitDB() (*sql.DB, error) {
	DB, err := sql.Open(
		"postgres",
		fmt.Sprintf("host=localhost port=5432 user=artemkashipov password=%s dbname=l0_data sslmode=disable", arguments.PostgresPWD),
	)
	if err != nil {
		return nil, fmt.Errorf("Problem with opening DB: %w", err)
	}
	err = DB.Ping()
	if err != nil {
		return nil, fmt.Errorf("Problem with pinging of DB: %w", err)
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

func CreateDefaultTables(db *sql.DB) error {
	filename := "init.sql"
	query, err := ReadQuery(filename)
	if err != nil {
		return fmt.Errorf("Problem with reading of '%s' query: %w", filename, err)
	}
	_, err = db.Exec(
		query,
	)
	if err != nil {
		return fmt.Errorf("Problem with execution of init query: %w", err)
	}
	return nil
}
