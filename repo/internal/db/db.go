package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func Init() {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	var err error
	for i := 0; i < 15; i++ {
		DB, err = sql.Open("mysql", dsn)
		if err == nil {
			if pingErr := DB.Ping(); pingErr == nil {
				DB.SetMaxOpenConns(25)
				DB.SetMaxIdleConns(10)
				DB.SetConnMaxLifetime(30 * time.Minute)
				log.Println("connected to MySQL")
				return
			}
		}
		log.Printf("waiting for MySQL... attempt %d/15", i+1)
		time.Sleep(3 * time.Second)
	}
	log.Fatalf("could not connect to MySQL: %v", err)
}

func Close() {
	if DB != nil {
		_ = DB.Close()
	}
}
