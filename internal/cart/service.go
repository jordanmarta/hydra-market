package cart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jordanmarta/hydra-market.git/internal/product"
	"github.com/jordanmarta/hydra-market.git/internal/user"
)

var (
	ErrInvalidQuantity  = errors.New("quantity must be greater than zero")
	ErrEmptyCart        = errors.New("cart must contain at least one item")
	ErrDuplicateProduct = errors.New("cart contains duplicated product")
	ErrUserNotFound     = errors.New("user not found")
	ErrProductNotFound  = errors.New("product not found")
	ErrCartNotFound     = errors.New("cart not found")
	ErrCartItemNotFound = errors.New("cart item not found")
	ErrCartNotActive    = errors.New("cart is not active")
	ErrActiveCartExists = errors.New("user already has an active cart")
)

type CreateItemInput struct {
	ProductID int64
	Quantity  int
}

type Service struct {
	db                *sql.DB
	repository        *Repository
	productRepository *product.Repository
	userRepository    *user.Repository
}

func NewService(
	db *sql.DB,
	repository *Repository,
	productRepository *product.Repository,
	userRepository *user.Repository,
) *Service {
	return &Service{
		db:                db,
		repository:        repository,
		productRepository: productRepository,
		userRepository:    userRepository,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID int64,
	inputItems []CreateItemInput,
) (*Cart, error) {

	if len(inputItems) == 0 {
		return nil, ErrEmptyCart
	}

	_, err := s.userRepository.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf("get user: %w", err)
	}

	activeCart, err := s.repository.GetActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get active cart: %w", err)
	}

	if activeCart != nil {
		return nil, ErrActiveCartExists
	}

	items := make([]CartItem, 0, len(inputItems))
	productsSeen := make(map[int64]struct{})

	for _, input := range inputItems {
		if input.Quantity <= 0 {
			return nil, ErrInvalidQuantity
		}

		if _, exists := productsSeen[input.ProductID]; exists {
			return nil, fmt.Errorf(
				"%w: product %d",
				ErrDuplicateProduct,
				input.ProductID,
			)
		}

		productsSeen[input.ProductID] = struct{}{}

		_, err := s.productRepository.GetByID(ctx, input.ProductID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf(
					"%w: product %d",
					ErrProductNotFound,
					input.ProductID,
				)
			}

			return nil, fmt.Errorf(
				"get product %d: %w",
				input.ProductID,
				err,
			)
		}

		items = append(items, CartItem{
			ProductID: input.ProductID,
			Quantity:  input.Quantity,
		})
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	cart := &Cart{
		UserID: userID,
		Status: "ACTIVE",
		Items:  items,
	}

	if err := s.repository.Create(ctx, tx, cart); err != nil {
		return nil, fmt.Errorf("create cart: %w", err)
	}

	if err := s.repository.CreateItems(
		ctx,
		tx,
		cart.ID,
		items,
	); err != nil {
		return nil, fmt.Errorf("create cart items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	cart, err = s.repository.GetByID(ctx, cart.ID)
	if err != nil {
		return nil, fmt.Errorf("reload cart: %w", err)
	}

	return cart, nil
}

func (s *Service) GetByID(
	ctx context.Context,
	cartID int64,
) (*Cart, error) {

	cart, err := s.repository.GetByID(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}

	if cart == nil {
		return nil, ErrCartNotFound
	}

	return cart, nil
}

func (s *Service) GetActiveByUserID(
	ctx context.Context,
	userID int64,
) (*Cart, error) {

	cart, err := s.repository.GetActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get active cart: %w", err)
	}

	if cart == nil {
		return nil, ErrCartNotFound
	}

	return cart, nil
}

func (s *Service) UpsertItemQuantity(
	ctx context.Context,
	cartID int64,
	productID int64,
	quantity int,
) error {

	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	cart, err := s.repository.GetByID(ctx, cartID)
	if err != nil {
		return fmt.Errorf("get cart: %w", err)
	}

	if cart == nil {
		return ErrCartNotFound
	}

	if cart.Status != "ACTIVE" {
		return ErrCartNotActive
	}

	_, err = s.productRepository.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProductNotFound
		}

		return fmt.Errorf("get product: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.repository.UpsertItemQuantity(
		ctx,
		tx,
		cartID,
		productID,
		quantity,
	); err != nil {
		return fmt.Errorf("upsert cart item quantity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Service) DeleteItem(
	ctx context.Context,
	cartID int64,
	productID int64,
) error {

	cart, err := s.repository.GetByID(ctx, cartID)
	if err != nil {
		return fmt.Errorf("get cart: %w", err)
	}

	if cart == nil {
		return ErrCartNotFound
	}

	if cart.Status != "ACTIVE" {
		return ErrCartNotActive
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	deleted, err := s.repository.DeleteItem(
		ctx,
		tx,
		cartID,
		productID,
	)
	if err != nil {
		return fmt.Errorf("delete cart item: %w", err)
	}

	if !deleted {
		return ErrCartItemNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
