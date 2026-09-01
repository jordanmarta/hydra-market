package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jordanmarta/hydra-market.git/internal/inventory"
	"github.com/jordanmarta/hydra-market.git/internal/product"
	"github.com/jordanmarta/hydra-market.git/internal/user"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type Service struct {
	db                  *sql.DB
	orderRepository     *Repository
	productRepository   *product.Repository
	inventoryRepository *inventory.Repository
	userRepository      *user.Repository
}

func NewService(
	db *sql.DB,
	orderRepository *Repository,
	productRepository *product.Repository,
	inventoryRepository *inventory.Repository,
	userRepository *user.Repository,
) *Service {
	return &Service{
		db:                  db,
		orderRepository:     orderRepository,
		productRepository:   productRepository,
		inventoryRepository: inventoryRepository,
		userRepository:      userRepository,
	}
}

func (s *Service) Create(ctx context.Context, productID int64, userID int64, quantity int) (*Order, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	product, err := s.productRepository.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stockDecreased, err := s.inventoryRepository.DecreaseStock(
		ctx,
		tx,
		productID,
		quantity,
	)
	if err != nil {
		return nil, fmt.Errorf("decreased stock: %w", err)
	}

	if !stockDecreased {
		return nil, ErrInsufficientStock
	}

	order := &Order{
		UserID: user.ID,
		Status: "CREATED",
		Items: []Item{
			{
				ProductID:    product.ID,
				Quantity:     quantity,
				UnitPrice:    product.Price,
				CurrencyCode: product.CurrencyCode,
			},
		},
	}

	if err := s.orderRepository.Create(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return order, nil
}
