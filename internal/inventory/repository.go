package inventory

import (
	"context"
	"database/sql"
	"errors"
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

func (r *Repository) SetStock(
	ctx context.Context,
	productID int64,
	quantity int,
) error {
	query := `
		INSERT INTO inventory (
			product_id,
			quantity
		)
		VALUES ($1, $2)
		ON CONFLICT (product_id)
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			updated_at = NOW();
	`

	_, err := r.db.ExecContext(ctx, query, productID, quantity)
	if err != nil {
		return fmt.Errorf("set inventory stock: %w", err)
	}

	return nil
}

func (r *Repository) GetStock(
	ctx context.Context,
	productID int64,
) (int, error) {
	query := `
		SELECT quantity
		FROM inventory
		WHERE product_id = $1;
	`

	var quantity int

	err := r.db.QueryRowContext(ctx, query, productID).Scan(&quantity)
	if err != nil {
		return 0, fmt.Errorf("get stock: %w", err)
	}

	return quantity, nil
}

func (r *Repository) DecreaseStock(
	ctx context.Context,
	productID int64,
	quantity int,
) (bool, error) {
	query := `
		UPDATE inventory
		SET
			quantity = quantity - $2,
			updated_at = NOW()
		WHERE product_id = $1
		  AND quantity >= $2
		RETURNING quantity;
	`

	var remainingStock int

	err := r.db.QueryRowContext(
		ctx,
		query,
		productID,
		quantity,
	).Scan(&remainingStock)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("decrease stock: %w", err)
	}

	return true, nil
}
