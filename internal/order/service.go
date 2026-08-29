package order

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jordanmarta/hydra-market.git/internal/inventory"
	"github.com/jordanmarta/hydra-market.git/internal/product"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type Service struct {
	orderRepository     *Repository
	productRepository   *product.Repository
	inventoryRepository *inventory.Repository
}

func NewService(
	orderRepository *Repository,
	productRepository *product.Repository,
	inventoryRepository *inventory.Repository,
) *Service {
	return &Service{
		orderRepository:     orderRepository,
		productRepository:   productRepository,
		inventoryRepository: inventoryRepository,
	}
}

func (s *Service) Create(ctx context.Context, productID int64, quantity int) (*Order, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	product, err := s.productRepository.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}

	currentStock, err := s.inventoryRepository.GetStock(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("get stock: %w", err)
	}

	log.Printf(
		"[RACE] READ  product=%d stock=%d",
		productID,
		currentStock,
	)

	if currentStock < quantity {
		return nil, ErrInsufficientStock
	}

	newStock := currentStock - quantity

	log.Printf(
		"[RACE] WRITE product=%d stock=%d",
		productID,
		newStock,
	)

	if err := s.inventoryRepository.SetStock(ctx, productID, newStock); err != nil {
		return nil, fmt.Errorf("update stock: %w", err)
	}

	order := &Order{
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

	if err := s.orderRepository.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return order, nil
}
