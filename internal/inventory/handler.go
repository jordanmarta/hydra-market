package inventory

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handler struct {
	repository *Repository
}

type setStockRequest struct {
	Quantity int `json:"quantity"`
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) SetStock(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var request setStockRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if request.Quantity < 0 {
		http.Error(w, "quantity cannot be negative", http.StatusBadRequest)
		return
	}

	if err := h.repository.SetStock(
		r.Context(),
		productID,
		request.Quantity,
	); err != nil {
		http.Error(w, "failed to set stock", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
