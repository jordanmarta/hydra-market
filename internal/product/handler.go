package product

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	repository *Repository
}

type createProductRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Price        int64   `json:"price"`
	CurrencyCode string  `json:"currency_code"`
}

type createProductResponse struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Price        int64   `json:"price"`
	CurrencyCode string  `json:"currency_code"`
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createProductRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	product := Product{
		Name:         request.Name,
		Description:  request.Description,
		Price:        request.Price,
		CurrencyCode: request.CurrencyCode,
	}

	if err := h.repository.Create(r.Context(), &product); err != nil {
		http.Error(w, "failed to create product", http.StatusInternalServerError)
		return
	}

	response := createProductResponse{
		ID:           product.ID,
		Name:         product.Name,
		Description:  product.Description,
		Price:        product.Price,
		CurrencyCode: product.CurrencyCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
