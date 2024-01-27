package payment

type Payment struct {
	TransactionID   string  `json:"transaction"`
	RequestID       string  `json:"request_id"`
	Currency        string  `json:"currency"`
	ProviderID      string  `json:"provider"`
	Amount          float64 `json:"amount"`
	PaymentDateTime int64   `json:"payment_dt"`
	Bank            string  `json:"bank"`
	DeliveryCost    float64 `json:"delivery_cost"`
	GoodsTotal      float64 `json:"goods_total"`
	CustomFee       float64 `json:"custom_fee"`
}
