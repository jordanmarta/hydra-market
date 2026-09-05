package order

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

type createOrderItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type createOrderRequest struct {
	UserID int64                    `json:"user_id"`
	Items  []createOrderItemRequest `json:"items"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createOrderRequest

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
	order, err := h.service.Create(
		r.Context(),
		request.UserID,
		items,
	)

	if err != nil {
		if errors.Is(err, ErrInsufficientStock) {
			http.Error(w, "insufficient stock", http.StatusConflict)
			return
		}

		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(order); err != nil {
		return
	}
}
