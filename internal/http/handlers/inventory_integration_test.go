package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andro-kes/gateway/internal/http/handlers"
	pbInv "github.com/andro-kes/inventory_service/proto"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// mockInventoryServiceClient is a mock implementation of pbInv.InventoryServiceClient
type mockInventoryServiceClient struct {
	pbInv.InventoryServiceClient
	createProductFunc func(ctx context.Context, in *pbInv.CreateRequest, opts ...grpc.CallOption) (*pbInv.CreateResponse, error)
	getProductFunc    func(ctx context.Context, in *pbInv.GetRequest, opts ...grpc.CallOption) (*pbInv.GetResponse, error)
	updateProductFunc func(ctx context.Context, in *pbInv.UpdateRequest, opts ...grpc.CallOption) (*pbInv.UpdateResponse, error)
	deleteProductFunc func(ctx context.Context, in *pbInv.DeleteRequest, opts ...grpc.CallOption) (*pbInv.DeleteResponse, error)
	listProductsFunc  func(ctx context.Context, in *pbInv.ListRequest, opts ...grpc.CallOption) (*pbInv.ListResponse, error)
}

func (m *mockInventoryServiceClient) CreateProduct(ctx context.Context, in *pbInv.CreateRequest, opts ...grpc.CallOption) (*pbInv.CreateResponse, error) {
	if m.createProductFunc != nil {
		return m.createProductFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("createProductFunc not implemented")
}

func (m *mockInventoryServiceClient) GetProduct(ctx context.Context, in *pbInv.GetRequest, opts ...grpc.CallOption) (*pbInv.GetResponse, error) {
	if m.getProductFunc != nil {
		return m.getProductFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("getProductFunc not implemented")
}

func (m *mockInventoryServiceClient) UpdateProduct(ctx context.Context, in *pbInv.UpdateRequest, opts ...grpc.CallOption) (*pbInv.UpdateResponse, error) {
	if m.updateProductFunc != nil {
		return m.updateProductFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("updateProductFunc not implemented")
}

func (m *mockInventoryServiceClient) DeleteProduct(ctx context.Context, in *pbInv.DeleteRequest, opts ...grpc.CallOption) (*pbInv.DeleteResponse, error) {
	if m.deleteProductFunc != nil {
		return m.deleteProductFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("deleteProductFunc not implemented")
}

func (m *mockInventoryServiceClient) ListProducts(ctx context.Context, in *pbInv.ListRequest, opts ...grpc.CallOption) (*pbInv.ListResponse, error) {
	if m.listProductsFunc != nil {
		return m.listProductsFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("listProductsFunc not implemented")
}

// setupInventoryTestRouter creates a test router with the inventory handlers
func setupInventoryTestRouter(mockClient pbInv.InventoryServiceClient) *chi.Mux {
	invManager := handlers.NewInvManager(mockClient)
	r := chi.NewRouter()

	r.Route("/inventory", func(r chi.Router) {
		r.Post("/create", invManager.CreateHandler)
		r.Post("/get", invManager.GetHandler)
		r.Post("/update", invManager.UpdateHandler)
		r.Post("/delete", invManager.DeleteHandler)
		r.Post("/list", invManager.ListHandler)
	})

	return r
}

// TestCreateHandler_Success tests successful product creation
func TestCreateHandler_Success(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		createProductFunc: func(ctx context.Context, in *pbInv.CreateRequest, opts ...grpc.CallOption) (*pbInv.CreateResponse, error) {
			assert.NotNil(t, in.Product)
			assert.Equal(t, "Test Product", in.Product.Name)
			assert.Equal(t, "Test Description", in.Product.Description)
			assert.Equal(t, 29.99, in.Product.Price)
			assert.Equal(t, int32(100), in.Product.Quantity)

			return &pbInv.CreateResponse{
				Product: &pbInv.Product{
					Id:          "prod-123",
					Name:        in.Product.Name,
					Description: in.Product.Description,
					Price:       in.Product.Price,
					Quantity:    in.Product.Quantity,
					Available:   true,
				},
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create request
	reqBody := map[string]any{
		"product": map[string]any{
			"name":        "Test Product",
			"description": "Test Description",
			"price":       29.99,
			"quantity":    100,
		},
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/inventory/create", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	product, ok := respBody["product"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "prod-123", product["id"])
	assert.Equal(t, "Test Product", product["name"])
	assert.Equal(t, "Test Description", product["description"])
}

// TestCreateHandler_InvalidJSON tests create with malformed JSON
func TestCreateHandler_InvalidJSON(t *testing.T) {
	mockClient := &mockInventoryServiceClient{}
	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Send invalid JSON
	resp, err := http.Post(ts.URL+"/inventory/create", "application/json", bytes.NewBuffer([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to decode request body")
}

// TestCreateHandler_GRPCFailure tests create when the gRPC call fails
func TestCreateHandler_GRPCFailure(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		createProductFunc: func(ctx context.Context, in *pbInv.CreateRequest, opts ...grpc.CallOption) (*pbInv.CreateResponse, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"product": map[string]any{
			"name": "Test Product",
		},
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/create", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to create product")
}

// TestGetHandler_Success tests successful product retrieval
func TestGetHandler_Success(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		getProductFunc: func(ctx context.Context, in *pbInv.GetRequest, opts ...grpc.CallOption) (*pbInv.GetResponse, error) {
			assert.Equal(t, "prod-456", in.Id)

			return &pbInv.GetResponse{
				Product: &pbInv.Product{
					Id:          "prod-456",
					Name:        "Retrieved Product",
					Description: "Product Description",
					Price:       49.99,
					Quantity:    50,
					Available:   true,
				},
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create request
	reqBody := map[string]any{
		"id": "prod-456",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/inventory/get", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	product, ok := respBody["product"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "prod-456", product["id"])
	assert.Equal(t, "Retrieved Product", product["name"])
}

// TestGetHandler_InvalidJSON tests get with malformed JSON
func TestGetHandler_InvalidJSON(t *testing.T) {
	mockClient := &mockInventoryServiceClient{}
	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Send invalid JSON
	resp, err := http.Post(ts.URL+"/inventory/get", "application/json", bytes.NewBuffer([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestGetHandler_GRPCFailure tests get when the gRPC call fails
func TestGetHandler_GRPCFailure(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		getProductFunc: func(ctx context.Context, in *pbInv.GetRequest, opts ...grpc.CallOption) (*pbInv.GetResponse, error) {
			return nil, fmt.Errorf("product not found")
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"id": "non-existent",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/get", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to get product")
}

// TestUpdateHandler_Success tests successful product update
func TestUpdateHandler_Success(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		updateProductFunc: func(ctx context.Context, in *pbInv.UpdateRequest, opts ...grpc.CallOption) (*pbInv.UpdateResponse, error) {
			assert.NotNil(t, in.Product)
			assert.Equal(t, "prod-789", in.Product.Id)
			assert.Equal(t, "Updated Product", in.Product.Name)
			assert.Equal(t, 59.99, in.Product.Price)

			return &pbInv.UpdateResponse{
				Product: &pbInv.Product{
					Id:          in.Product.Id,
					Name:        in.Product.Name,
					Description: "Updated Description",
					Price:       in.Product.Price,
					Quantity:    75,
					Available:   true,
				},
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create request
	reqBody := map[string]any{
		"product": map[string]any{
			"id":    "prod-789",
			"name":  "Updated Product",
			"price": 59.99,
		},
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/inventory/update", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	product, ok := respBody["product"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "prod-789", product["id"])
	assert.Equal(t, "Updated Product", product["name"])
}

// TestUpdateHandler_InvalidJSON tests update with malformed JSON
func TestUpdateHandler_InvalidJSON(t *testing.T) {
	mockClient := &mockInventoryServiceClient{}
	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Send invalid JSON
	resp, err := http.Post(ts.URL+"/inventory/update", "application/json", bytes.NewBuffer([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestUpdateHandler_GRPCFailure tests update when the gRPC call fails
func TestUpdateHandler_GRPCFailure(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		updateProductFunc: func(ctx context.Context, in *pbInv.UpdateRequest, opts ...grpc.CallOption) (*pbInv.UpdateResponse, error) {
			return nil, fmt.Errorf("update failed")
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"product": map[string]any{
			"id":   "prod-789",
			"name": "Test",
		},
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/update", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to update product")
}

// TestDeleteHandler_Success tests successful product deletion
func TestDeleteHandler_Success(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		deleteProductFunc: func(ctx context.Context, in *pbInv.DeleteRequest, opts ...grpc.CallOption) (*pbInv.DeleteResponse, error) {
			assert.Equal(t, "prod-999", in.Id)

			return &pbInv.DeleteResponse{
				Success: true,
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create request
	reqBody := map[string]any{
		"id": "prod-999",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/inventory/delete", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	success, ok := respBody["success"].(bool)
	require.True(t, ok)
	assert.True(t, success)
}

// TestDeleteHandler_InvalidJSON tests delete with malformed JSON
func TestDeleteHandler_InvalidJSON(t *testing.T) {
	mockClient := &mockInventoryServiceClient{}
	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Send invalid JSON
	resp, err := http.Post(ts.URL+"/inventory/delete", "application/json", bytes.NewBuffer([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestDeleteHandler_GRPCFailure tests delete when the gRPC call fails
func TestDeleteHandler_GRPCFailure(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		deleteProductFunc: func(ctx context.Context, in *pbInv.DeleteRequest, opts ...grpc.CallOption) (*pbInv.DeleteResponse, error) {
			return nil, fmt.Errorf("delete failed")
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"id": "prod-999",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/delete", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to delete product")
}

// TestListHandler_Success tests successful product listing
func TestListHandler_Success(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		listProductsFunc: func(ctx context.Context, in *pbInv.ListRequest, opts ...grpc.CallOption) (*pbInv.ListResponse, error) {
			assert.Equal(t, int32(10), in.PageSize)
			assert.Equal(t, "name", in.OrderBy)

			return &pbInv.ListResponse{
				Products: []*pbInv.Product{
					{
						Id:          "prod-1",
						Name:        "Product 1",
						Description: "Description 1",
						Price:       19.99,
						Quantity:    10,
						Available:   true,
					},
					{
						Id:          "prod-2",
						Name:        "Product 2",
						Description: "Description 2",
						Price:       29.99,
						Quantity:    20,
						Available:   true,
					},
				},
				TotalSize: 2,
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create request
	reqBody := map[string]any{
		"page_size": 10,
		"order_by":  "name",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/inventory/list", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	products, ok := respBody["products"].([]any)
	require.True(t, ok)
	assert.Len(t, products, 2)

	totalSize, ok := respBody["total_size"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(2), totalSize)

	// Check first product
	firstProduct, ok := products[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "prod-1", firstProduct["id"])
	assert.Equal(t, "Product 1", firstProduct["name"])
}

// TestListHandler_InvalidJSON tests list with malformed JSON
func TestListHandler_InvalidJSON(t *testing.T) {
	mockClient := &mockInventoryServiceClient{}
	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Send invalid JSON
	resp, err := http.Post(ts.URL+"/inventory/list", "application/json", bytes.NewBuffer([]byte("invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestListHandler_GRPCFailure tests list when the gRPC call fails
func TestListHandler_GRPCFailure(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		listProductsFunc: func(ctx context.Context, in *pbInv.ListRequest, opts ...grpc.CallOption) (*pbInv.ListResponse, error) {
			return nil, fmt.Errorf("database query failed")
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"page_size": 10,
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/list", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to list products")
}

// TestListHandler_EmptyList tests list when no products are returned
func TestListHandler_EmptyList(t *testing.T) {
	mockClient := &mockInventoryServiceClient{
		listProductsFunc: func(ctx context.Context, in *pbInv.ListRequest, opts ...grpc.CallOption) (*pbInv.ListResponse, error) {
			return &pbInv.ListResponse{
				Products:  []*pbInv.Product{},
				TotalSize: 0,
			}, nil
		},
	}

	router := setupInventoryTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	reqBody := map[string]any{
		"page_size": 10,
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(ts.URL+"/inventory/list", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody map[string]any
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)

	// Proto3 omits empty fields, so products may not be present in the response
	if products, ok := respBody["products"].([]any); ok {
		assert.Len(t, products, 0)
	}
	// If products is not in the map, that's also acceptable for an empty list

	// total_size may also be omitted if zero in proto3, or it may be present
	if totalSize, ok := respBody["total_size"].(float64); ok {
		assert.Equal(t, float64(0), totalSize)
	}
}
