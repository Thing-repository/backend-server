package core

type UserDB struct {
	User
	PasswordHash string `json:"password_hash"`
}
