package order

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Create(ctx context.Context, tx *sql.Tx, order *Order) error {
	query := `
		INSERT INTO orders (status)
		VALUES ($1)
		RETURNING id;
	`

	if err := tx.QueryRowContext(ctx, query, order.Status).Scan(&order.ID); err != nil {
		return fmt.Errorf("create order: %w", err)
	}

	for i := range order.Items {
		item := &order.Items[i]

		itemQuery := `
			INSERT INTO order_items (
				order_id,
				product_id,
				quantity,
				unit_price,
				currency_code
			)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id;
		`

		if err := tx.QueryRowContext(
			ctx,
			itemQuery,
			order.ID,
			item.ProductID,
			item.Quantity,
			item.UnitPrice,
			item.CurrencyCode,
		).Scan(&item.ID); err != nil {
			return fmt.Errorf("create order item: %w", err)
		}
	}

	return nil
}
