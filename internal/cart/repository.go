package cart

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

func (r *Repository) GetByID(
	ctx context.Context,
	id int64,
) (*Cart, error) {

	query := `
		SELECT id, user_id, status
		FROM carts
		WHERE id = $1;
	`

	cart := &Cart{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.Status,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("get cart by id: %w", err)
	}

	if err := r.loadItems(ctx, cart); err != nil {
		return nil, err
	}

	return cart, nil
}

func (r *Repository) GetActiveByUserID(
	ctx context.Context,
	userID int64,
) (*Cart, error) {

	query := `
		SELECT id, user_id, status
		FROM carts
		WHERE user_id = $1
		  AND status = 'ACTIVE'
		ORDER BY created_at DESC
		LIMIT 1;
	`

	cart := &Cart{}

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&cart.ID,
		&cart.UserID,
		&cart.Status,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("get active cart by user id: %w", err)
	}

	if err := r.loadItems(ctx, cart); err != nil {
		return nil, err
	}

	return cart, nil
}

func (r *Repository) loadItems(
	ctx context.Context,
	cart *Cart,
) error {

	query := `
		SELECT product_id, quantity
		FROM cart_items
		WHERE cart_id = $1
		ORDER BY product_id;
	`

	rows, err := r.db.QueryContext(ctx, query, cart.ID)
	if err != nil {
		return fmt.Errorf("get cart items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := CartItem{
			CartID: cart.ID,
		}

		if err := rows.Scan(
			&item.ProductID,
			&item.Quantity,
		); err != nil {
			return fmt.Errorf("scan cart item: %w", err)
		}

		cart.Items = append(cart.Items, item)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate cart items: %w", err)
	}

	return nil
}

func (r *Repository) Create(
	ctx context.Context,
	tx *sql.Tx,
	cart *Cart,
) error {

	query := `
		INSERT INTO carts (
			user_id,
			status
		)
		VALUES ($1, $2)
		RETURNING id;
	`

	err := tx.QueryRowContext(
		ctx,
		query,
		cart.UserID,
		cart.Status,
	).Scan(&cart.ID)

	if err != nil {
		return fmt.Errorf("create cart: %w", err)
	}

	return nil
}

func (r *Repository) CreateItems(
	ctx context.Context,
	tx *sql.Tx,
	cartID int64,
	items []CartItem,
) error {

	query := `
		INSERT INTO cart_items (
			cart_id,
			product_id,
			quantity
		)
		VALUES ($1, $2, $3);
	`

	for _, item := range items {
		_, err := tx.ExecContext(
			ctx,
			query,
			cartID,
			item.ProductID,
			item.Quantity,
		)

		if err != nil {
			return fmt.Errorf(
				"create cart item for product %d: %w",
				item.ProductID,
				err,
			)
		}
	}

	return nil
}

func (r *Repository) UpsertItemQuantity(
	ctx context.Context,
	tx *sql.Tx,
	cartID int64,
	productID int64,
	quantity int,
) error {

	query := `
		INSERT INTO cart_items (
			cart_id,
			product_id,
			quantity
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, product_id)
		DO UPDATE SET
			quantity = EXCLUDED.quantity;
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		cartID,
		productID,
		quantity,
	)

	if err != nil {
		return fmt.Errorf("upsert cart item quantity: %w", err)
	}

	if err := r.touch(ctx, tx, cartID); err != nil {
		return err
	}

	return nil
}

func (r *Repository) DeleteItem(
	ctx context.Context,
	tx *sql.Tx,
	cartID int64,
	productID int64,
) (bool, error) {

	query := `
		DELETE FROM cart_items
		WHERE cart_id = $1
		  AND product_id = $2;
	`

	result, err := tx.ExecContext(
		ctx,
		query,
		cartID,
		productID,
	)

	if err != nil {
		return false, fmt.Errorf("delete cart item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return false, nil
	}

	if err := r.touch(ctx, tx, cartID); err != nil {
		return false, err
	}

	return true, nil
}

func (r *Repository) touch(
	ctx context.Context,
	tx *sql.Tx,
	cartID int64,
) error {

	query := `
		UPDATE carts
		SET updated_at = NOW()
		WHERE id = $1;
	`

	if _, err := tx.ExecContext(ctx, query, cartID); err != nil {
		return fmt.Errorf("update cart timestamp: %w", err)
	}

	return nil
}
