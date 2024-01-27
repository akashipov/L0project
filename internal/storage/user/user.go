package user

type User struct {
	Name        string `json:"name"`
	Phonenumber string `json:"phone"`
	Email       string `json:"email"`
	Address
	AddressID int64 `json:"address_id,omitempty"`
}

type Address struct {
	Zipcode string `json:"zip"`
	City    string `json:"city"`
	Address string `json:"address"`
	Region  string `json:"region"`
}
