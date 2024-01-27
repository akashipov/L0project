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
	"github.com/akashipov/L0project/internal/storage/order"
	"github.com/akashipov/L0project/internal/storage/user"
	_ "github.com/lib/pq"
)

type SqlWorker struct {
	DB *sql.DB
}

func NewSqlWorker() (*SqlWorker, error) {
	DB, err := InitDB()
	if err != nil {
		return nil, fmt.Errorf("Problem with init DB -> %w", err)
	}
	return &SqlWorker{DB: DB}, nil
}

func (w *SqlWorker) AddOrder(tx *sql.Tx, ctx context.Context, ord order.Order) error {
	var err error
	query := "INSERT INTO orders(order_id, track_number, entry, delivery_user, " +
		"transaction_id, locale, internal_signature, customer_id, delivery_service, shardkey," +
		"sm_id, oof_shard, date_created) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)"
	var transID *string
	if ord.PaymentInfo == nil {
		transID = nil
	} else {
		transID = &ord.PaymentInfo.TransactionID
	}

	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Phonenumber,
			transID, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)
		if err != nil {
			return fmt.Errorf("Problem with execution of Add Order query: %w", err)
		}
	} else {
		_, err = tx.ExecContext(
			ctx, query, ord.OrderID,
			ord.TrackNumber, ord.Entry, ord.User.Email,
			nil, ord.Locale, ord.InternalSignature, ord.CustomerID, ord.DeliveryService,
			ord.ShardKey, ord.SmID, ord.OofShard, ord.DateCreated,
		)
		if err != nil {
			rollErr := tx.Rollback()
			return fmt.Errorf("Problem with execution of Add Order query: %w", errors.Join(err, rollErr))
		}
	}
	return nil
}

func (w *SqlWorker) AddUser(tx *sql.Tx, ctx context.Context, user user.User) error {
	var err error
	query := "INSERT INTO users(phonenumber, name, zipcode, city, address_id, region, email) VALUES($1, $2, $3, $4, $5, $6, $7)"
	if tx == nil {
		_, err = w.DB.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Zipcode,
			user.City, user.Address, user.Region, user.Email,
		)
		if err != nil {
			return fmt.Errorf("Problem with execution of Add User query: %w", err)
		}
	} else {
		_, err = tx.ExecContext(
			ctx, query, user.Phonenumber,
			user.Name, user.Zipcode,
			user.City, user.Address, user.Region, user.Email,
		)
		if err != nil {
			rollErr := tx.Rollback()
			return fmt.Errorf("Problem with execution of Add User query: %w", errors.Join(err, rollErr))
		}
	}
	return nil
}

// func (w *SqlWorker) AddItems(tx *sql.Tx, ctx context.Context, user user.User) error {

// }

func (w *SqlWorker) AddItem() {

}

func (w *SqlWorker) AddPaymentInfo() {

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

func CreateDefaultTables(db *sql.DB) error {
	d, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Problem with get pwd: %w", err)
	}
	filename := "init.sql"
	filePath := filepath.Join(
		d,
		"internal", "storage", "postgres",
		"resources", filename,
	)
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0000)
	if err != nil {
		return fmt.Errorf("Problem with opening '%s' file: %w", filename, err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Problem with reading '%s' file: %w", filename, err)
	}
	_, err = db.Exec(
		string(b),
	)
	if err != nil {
		return fmt.Errorf("Problem with execution of init query: %w", err)
	}
	return nil
}
