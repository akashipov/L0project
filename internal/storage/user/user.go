package user

type User struct {
	Name        string `json:"name"`
	Phonenumber string `json:"phone"`
	Zipcode     string `json:"zip"`
	City        string `json:"city"`
	Address     string `json:"address"`
	Region      string `json:"region"`
	Email       string `json:"email"`
}
