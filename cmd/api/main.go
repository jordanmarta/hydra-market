package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jordanmarta/hydra-market.git/internal/product"
)

func main() {
	db, err := sql.Open(
		"pgx",
		"postgres://hydra:hydra@localhost:5432/hydra_market?sslmode=disable",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("database connection established")

	productRepository := product.NewRepository(db)
	productHandler := product.NewHandler(productRepository)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hydra is alive"))
	})

	mux.HandleFunc("POST /products", productHandler.Create)

	fmt.Println("hydra-market listening on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
