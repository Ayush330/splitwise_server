package main;


import(
	"log"
	"time"
	"net/http"
)

func main(){
	log.Println("Creating the splitwise")
	server := &http.Server{
		Addr: ":8080",
		Handler: nil,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	server.ListenAndServe()
}

