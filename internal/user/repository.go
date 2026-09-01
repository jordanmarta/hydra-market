package user

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

func (r *Repository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			name,
			email,
			address
		)
		VALUES ($1, $2, $3)
		RETURNING id;
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		user.Name,
		user.Email,
		user.Address,
	).Scan(&user.ID)

	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `
		SELECT
			id,
			name,
			email,
			address
		FROM users
		WHERE id = $1;
	`

	var user User

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Address,
	)

	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &user, nil
}
