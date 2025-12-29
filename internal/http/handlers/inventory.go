package handlers

import (
	"encoding/json"
	"net/http"

	pbInv "github.com/andro-kes/inventory_service/proto"
)

type InvManager struct {
	client pbInv.InventoryServiceClient
}

func NewInvManager(client pbInv.InventoryServiceClient) *InvManager {
	return &InvManager{
		client: client,
	}
}

func (im *InvManager) CreateHandler(w http.ResponseWriter, r *http.Request) {
	var req pbInv.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	product, err := im.client.CreateProduct(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to create product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		http.Error(w, "failed to encode result", http.StatusInternalServerError)
	}
}

func (im *InvManager) GetHandler(w http.ResponseWriter, r *http.Request) {
	var req pbInv.GetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	p, err := im.client.GetProduct(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to get product", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		http.Error(w, "failed to encode result", http.StatusInternalServerError)
		return
	}
}

func (im *InvManager) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	var req pbInv.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	p, err := im.client.UpdateProduct(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to update product", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		http.Error(w, "failed to encode result", http.StatusInternalServerError)
		return
	}
}

func (im *InvManager) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	var req pbInv.DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	resp, err := im.client.DeleteProduct(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to delete product", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode result", http.StatusInternalServerError)
		return
	}
}

func (im *InvManager) ListHandler(w http.ResponseWriter, r *http.Request) {
	var req pbInv.ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}

	resp, err := im.client.ListProducts(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode result", http.StatusInternalServerError)
		return
	}
}
