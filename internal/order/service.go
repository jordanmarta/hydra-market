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

type CreateItemInput struct {
	ProductID int64
	Quantity  int
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

func (s *Service) Create(
	ctx context.Context,
	userID int64,
	inputItems []CreateItemInput,
) (*Order, error) {

	if len(inputItems) == 0 {
		return nil, fmt.Errorf("order must contain at least one item")
	}

	user, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	items := make([]Item, 0, len(inputItems))

	for _, input := range inputItems {
		if input.Quantity <= 0 {
			return nil, fmt.Errorf("quantity must be greater than zero")
		}

		product, err := s.productRepository.GetByID(ctx, input.ProductID)
		if err != nil {
			return nil, fmt.Errorf("get product %d: %w", input.ProductID, err)
		}

		items = append(items, Item{
			ProductID:    product.ID,
			Quantity:     input.Quantity,
			UnitPrice:    product.Price,
			CurrencyCode: product.CurrencyCode,
		})
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, item := range items {
		stockDecreased, err := s.inventoryRepository.DecreaseStock(
			ctx,
			tx,
			item.ProductID,
			item.Quantity,
		)
		if err != nil {
			return nil, fmt.Errorf("decrease stock: %w", err)
		}

		if !stockDecreased {
			return nil, ErrInsufficientStock
		}
	}

	order := &Order{
		UserID: user.ID,
		Status: "CREATED",
		Items:  items,
	}

	if err := s.orderRepository.Create(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return order, nil
}
