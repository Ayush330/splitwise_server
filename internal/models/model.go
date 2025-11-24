package models




type User struct{
	Id string `json:"id"`
	Uuid string `json:"uuid"`
	Email string `json:"email"`
	Name string `json:"name"`
	Password string `json:"password"`
}

type CreateUserRequest struct{
	Email string `json:"email"`
	Name string `json:"name"`
	Password string `json:"password"`
}
