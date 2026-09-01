package user

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	repository *Repository
}

type createUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

type createUserResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user := User{
		Name:    request.Name,
		Email:   request.Email,
		Address: request.Address,
	}

	if err := h.repository.Create(r.Context(), &user); err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	response := createUserResponse{
		ID:      user.ID,
		Name:    user.Name,
		Email:   user.Email,
		Address: user.Address,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
