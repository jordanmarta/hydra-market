package cart

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Handler struct {
	service *Service
}

type createCartItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type createCartRequest struct {
	UserID int64                   `json:"user_id"`
	Items  []createCartItemRequest `json:"items"`
}

type updateItemRequest struct {
	Quantity int `json:"quantity"`
}

type cartItemResponse struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type cartResponse struct {
	ID     int64              `json:"id"`
	UserID int64              `json:"user_id"`
	Status string             `json:"status"`
	Items  []cartItemResponse `json:"items"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request createCartRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	items := make([]CreateItemInput, 0, len(request.Items))

	for _, item := range request.Items {
		items = append(items, CreateItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	cart, err := h.service.Create(
		r.Context(),
		request.UserID,
		items,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyCart),
			errors.Is(err, ErrInvalidQuantity),
			errors.Is(err, ErrDuplicateProduct):
			http.Error(w, err.Error(), http.StatusBadRequest)

		case errors.Is(err, ErrUserNotFound),
			errors.Is(err, ErrProductNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)

		case errors.Is(err, ErrActiveCartExists):
			http.Error(w, err.Error(), http.StatusConflict)

		default:
			http.Error(w, "failed to create cart", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(toCartResponse(cart)); err != nil {
		return
	}
}

func (h *Handler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	cartID, err := strconv.ParseInt(
		r.PathValue("cartId"),
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid cart id", http.StatusBadRequest)
		return
	}

	cart, err := h.service.GetByID(
		r.Context(),
		cartID,
	)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get cart", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(toCartResponse(cart)); err != nil {
		return
	}
}

func (h *Handler) GetActiveByUserID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userIDParam := r.URL.Query().Get("user_id")

	if userIDParam == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(
		userIDParam,
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	status := r.URL.Query().Get("status")

	if status != "" && status != "ACTIVE" {
		http.Error(w, "unsupported cart status", http.StatusBadRequest)
		return
	}

	cart, err := h.service.GetActiveByUserID(
		r.Context(),
		userID,
	)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get cart", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(toCartResponse(cart)); err != nil {
		return
	}
}

func (h *Handler) UpsertItemQuantity(
	w http.ResponseWriter,
	r *http.Request,
) {
	cartID, err := strconv.ParseInt(
		r.PathValue("cartId"),
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid cart id", http.StatusBadRequest)
		return
	}

	productID, err := strconv.ParseInt(
		r.PathValue("productId"),
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var request updateItemRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	err = h.service.UpsertItemQuantity(
		r.Context(),
		cartID,
		productID,
		request.Quantity,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidQuantity):
			http.Error(w, err.Error(), http.StatusBadRequest)

		case errors.Is(err, ErrCartNotFound),
			errors.Is(err, ErrProductNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)

		case errors.Is(err, ErrCartNotActive):
			http.Error(w, err.Error(), http.StatusConflict)

		default:
			http.Error(w, "failed to update cart item", http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteItem(
	w http.ResponseWriter,
	r *http.Request,
) {
	cartID, err := strconv.ParseInt(
		r.PathValue("cartId"),
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid cart id", http.StatusBadRequest)
		return
	}

	productID, err := strconv.ParseInt(
		r.PathValue("productId"),
		10,
		64,
	)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	err = h.service.DeleteItem(
		r.Context(),
		cartID,
		productID,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrCartNotFound),
			errors.Is(err, ErrCartItemNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)

		case errors.Is(err, ErrCartNotActive):
			http.Error(w, err.Error(), http.StatusConflict)

		default:
			http.Error(w, "failed to delete cart item", http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toCartResponse(cart *Cart) cartResponse {
	items := make([]cartItemResponse, 0, len(cart.Items))

	for _, item := range cart.Items {
		items = append(items, cartItemResponse{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return cartResponse{
		ID:     cart.ID,
		UserID: cart.UserID,
		Status: cart.Status,
		Items:  items,
	}
}
