package handlers


import(
	_"log"
	"net/http"
	"github.com/Ayush330/splitwise_server/internal/models"
	"encoding/json"
	"github.com/Ayush330/splitwise_server/internal/database"
)

func Login(res http.ResponseWriter, req *http.Request){
        res.WriteHeader(http.StatusOK)
        res.Write([]byte("Logged In..."))
}

func Logout(res http.ResponseWriter, req *http.Request){
        res.WriteHeader(http.StatusOK)
        res.Write([]byte("Logged Out..."))
}

func CreateUser(res http.ResponseWriter, req *http.Request){
	var user models.CreateUserRequest
	if err := json.NewDecoder(req.Body).Decode(&user); err != nil{
		http.Error(res, "Invalid Json", http.StatusBadRequest)
		return
	}
	query := "INSERT INTO users (email, name, password) VALUES (?, ?, ?)"
	_, err := database.DB.Exec(query, user.Email, user.Name, user.Password)
	if err != nil{
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("Bad Body"))
		return 
	}
	res.WriteHeader(http.StatusCreated)
	res.Write([]byte("Received user: " + user.Name))
}
