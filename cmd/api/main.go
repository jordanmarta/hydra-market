package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jordanmarta/hydra-market.git/internal/inventory"
	"github.com/jordanmarta/hydra-market.git/internal/order"
	"github.com/jordanmarta/hydra-market.git/internal/product"
	"github.com/jordanmarta/hydra-market.git/internal/user"
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
	inventoryRepository := inventory.NewRepository(db)
	orderRepository := order.NewRepository()
	userRepository := user.NewRepository(db)

	orderService := order.NewService(
		db,
		orderRepository,
		productRepository,
		inventoryRepository,
		userRepository,
	)

	orderHandler := order.NewHandler(orderService)
	productHandler := product.NewHandler(productRepository)
	inventoryHandler := inventory.NewHandler(inventoryRepository)
	userHandler := user.NewHandler(userRepository)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hydra is alive"))
	})

	mux.HandleFunc("POST /products", productHandler.Create)
	mux.HandleFunc("PUT /inventory/{id}", inventoryHandler.SetStock)
	mux.HandleFunc("POST /orders", orderHandler.Create)
	mux.HandleFunc("POST /users", userHandler.Create)
	mux.HandleFunc("GET /users/{id}", userHandler.GetByID)

	fmt.Println("hydra-market listening on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
