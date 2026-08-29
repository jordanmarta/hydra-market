package product

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Create(ctx context.Context, product *Product) error {
	query := `
		INSERT INTO products (
			name,
			description,
			price,
			currency_code
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.CurrencyCode,
	).Scan(&product.ID)

	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Product, error) {
	query := `
		SELECT
			id,
			name,
			description,
			price,
			currency_code
		FROM products
		WHERE id = $1;
	`

	var product Product

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&product.ID,
		&product.Name,
		&product.Description,
		&product.Price,
		&product.CurrencyCode,
	)

	if err != nil {
		return nil, fmt.Errorf("get product by id: %w", err)
	}

	return &product, nil
}
