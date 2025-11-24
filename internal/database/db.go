package database

import(
	"database/sql"
	"log"
	"time"
	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func Connect(){
	var err error
	dsn := "ayush:ayush123@tcp(127.0.0.1:3306)/splitwise"
	DB, err = sql.Open("mysql", dsn)
	if err != nil{
		log.Fatal(err)
	}
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)
	if err := DB.Ping(); err != nil {
		log.Fatal("Could not connect to database:", err)
	}
	log.Println("Successfully connected to MySQL with pooling enabled!")
}
