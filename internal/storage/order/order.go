package order

import (
	"github.com/akashipov/L0project/internal/storage/item"
	"github.com/akashipov/L0project/internal/storage/payment"
	"github.com/akashipov/L0project/internal/storage/user"
)

type Order struct {
	OrderID           string           `json:"order_uid"`
	Entry             string           `json:"entry"`
	TrackNumber       string           `json:"track_number"`
	Locale            string           `json:"locale"`
	InternalSignature string           `json:"internal_signature"`
	CustomerID        string           `json:"customer_id"`
	DeliveryService   string           `json:"delivery_service"`
	ShardKey          string           `json:"shardkey"`
	SmID              int32            `json:"sm_id"`
	DateCreated       string           `json:"date_created"`
	OofShard          string           `json:"oof_shard"`
	User              *user.User       `json:"delivery"`
	PaymentInfo       *payment.Payment `json:"payment"`
	Items             []item.Item      `json:"items"`
}

func NewOrder() Order {
	var ord Order
	var usr user.User
	var payInfo payment.Payment
	ord.User = &usr
	ord.PaymentInfo = &payInfo
	items := make([]item.Item, 0)
	ord.Items = items
	return ord
}
