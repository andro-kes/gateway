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
	"time"

	pb "github.com/andro-kes/auth_service/proto"
	"github.com/andro-kes/gateway/internal/http/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

// mockAuthServiceClient is a mock implementation of pb.AuthServiceClient
type mockAuthServiceClient struct {
	pb.AuthServiceClient
	loginFunc    func(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error)
	registerFunc func(ctx context.Context, in *pb.RegisterRequest, opts ...grpc.CallOption) (*pb.RegisterResponse, error)
	refreshFunc  func(ctx context.Context, in *pb.RefreshRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error)
	revokeFunc   func(ctx context.Context, in *pb.RevokeRequest, opts ...grpc.CallOption) (*pb.RevokeResponse, error)
}

func (m *mockAuthServiceClient) Login(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("loginFunc not implemented")
}

func (m *mockAuthServiceClient) Register(ctx context.Context, in *pb.RegisterRequest, opts ...grpc.CallOption) (*pb.RegisterResponse, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("registerFunc not implemented")
}

func (m *mockAuthServiceClient) Refresh(ctx context.Context, in *pb.RefreshRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error) {
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("refreshFunc not implemented")
}

func (m *mockAuthServiceClient) Revoke(ctx context.Context, in *pb.RevokeRequest, opts ...grpc.CallOption) (*pb.RevokeResponse, error) {
	if m.revokeFunc != nil {
		return m.revokeFunc(ctx, in, opts...)
	}
	return nil, fmt.Errorf("revokeFunc not implemented")
}

// setupTestRouter creates a test router with the auth handlers
func setupTestRouter(mockClient pb.AuthServiceClient) *chi.Mux {
	authManager := handlers.NewAuthManager(mockClient)
	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authManager.LoginHandler)
		r.Post("/register", authManager.RegisterHandler)
		r.Post("/refresh", authManager.RefreshHandler)
		r.Post("/revoke", authManager.RevokeHandler)
	})

	// Add a protected route to test the middleware
	r.Group(func(r chi.Router) {
		r.Use(handlers.PropagateAuthToGRPC)
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "success"})
		})
	})

	return r
}

// generateMockJWT creates a simple JWT-like token for testing
// This is just for testing - it's not cryptographically valid
func generateMockJWT(expiry time.Time) string {
	// JWT structure: header.payload.signature
	// We only need a valid payload with exp claim for the middleware to parse
	header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"HS256","typ":"JWT"}
	
	payload := map[string]interface{}{
		"exp": expiry.Unix(),
		"sub": "test-user-123",
	}
	payloadJSON, _ := json.Marshal(payload)
	// Use base64 URL encoding without padding
	payloadB64 := base64URLEncode(payloadJSON)
	
	signature := "test-signature"
	
	return fmt.Sprintf("%s.%s.%s", header, payloadB64, signature)
}

func base64URLEncode(data []byte) string {
	// Implement base64 URL encoding without padding
	const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var result []byte
	
	for i := 0; i < len(data); i += 3 {
		var b1, b2, b3 byte
		b1 = data[i]
		if i+1 < len(data) {
			b2 = data[i+1]
		}
		if i+2 < len(data) {
			b3 = data[i+2]
		}
		
		result = append(result, encodeURL[(b1>>2)&0x3F])
		result = append(result, encodeURL[((b1<<4)|(b2>>4))&0x3F])
		
		if i+1 < len(data) {
			result = append(result, encodeURL[((b2<<2)|(b3>>6))&0x3F])
		}
		if i+2 < len(data) {
			result = append(result, encodeURL[b3&0x3F])
		}
	}
	
	return string(result)
}

// TestLoginHandler_Success tests successful authentication
func TestLoginHandler_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		loginFunc: func(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error) {
			assert.Equal(t, "testuser", in.Username)
			assert.Equal(t, "testpass", in.Password)
			
			return &pb.TokenResponse{
				UserId:           "user-123",
				AccessToken:      generateMockJWT(time.Now().Add(5 * time.Minute)),
				RefreshToken:     "refresh-token-xyz",
				AccessExpiresIn:  durationpb.New(5 * time.Minute),
				RefreshExpiresIn: durationpb.New(24 * time.Hour),
			}, nil
		},
	}

	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create login request
	reqBody := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "user-123", respBody["user_id"])
	assert.NotEmpty(t, respBody["access_token"])
	assert.NotNil(t, respBody["access_expires_in_seconds"])

	// Check cookies
	cookies := resp.Cookies()
	var accessCookie, refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			accessCookie = c
		}
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, accessCookie, "access_token cookie should be set")
	assert.NotNil(t, refreshCookie, "refresh_token cookie should be set")
	assert.True(t, accessCookie.HttpOnly)
	assert.True(t, refreshCookie.HttpOnly)

	// Check Authorization header
	authHeader := resp.Header.Get("Authorization")
	assert.NotEmpty(t, authHeader)
	assert.Contains(t, authHeader, "Bearer ")
}

// TestLoginHandler_InvalidCredentials tests failed authentication
func TestLoginHandler_InvalidCredentials(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		loginFunc: func(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error) {
			return nil, fmt.Errorf("invalid credentials")
		},
	}

	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create login request with wrong credentials
	reqBody := map[string]string{
		"username": "testuser",
		"password": "wrongpass",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "invalid credentials")
}

// TestLoginHandler_MissingCredentials tests missing username/password
func TestLoginHandler_MissingCredentials(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	tests := []struct {
		name     string
		reqBody  map[string]string
		wantCode int
	}{
		{
			name:     "missing username",
			reqBody:  map[string]string{"password": "testpass"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing password",
			reqBody:  map[string]string{"username": "testuser"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty credentials",
			reqBody:  map[string]string{"username": "", "password": ""},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqJSON, err := json.Marshal(tt.reqBody)
			require.NoError(t, err)

			resp, err := http.Post(ts.URL+"/auth/login", "application/json", bytes.NewBuffer(reqJSON))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

// TestRegisterHandler_Success tests successful user registration
func TestRegisterHandler_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		registerFunc: func(ctx context.Context, in *pb.RegisterRequest, opts ...grpc.CallOption) (*pb.RegisterResponse, error) {
			assert.Equal(t, "newuser", in.Username)
			assert.Equal(t, "newpass", in.Password)
			
			return &pb.RegisterResponse{
				UserId: "user-456",
			}, nil
		},
	}

	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create register request
	reqBody := map[string]string{
		"username": "newuser",
		"password": "newpass",
		"email":    "newuser@example.com",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/auth/register", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "user-456", respBody["user_id"])
}

// TestRefreshHandler_Success tests successful token refresh
func TestRefreshHandler_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		refreshFunc: func(ctx context.Context, in *pb.RefreshRequest, opts ...grpc.CallOption) (*pb.TokenResponse, error) {
			assert.Equal(t, "refresh-token-xyz", in.RefreshToken)
			
			return &pb.TokenResponse{
				UserId:           "user-123",
				AccessToken:      generateMockJWT(time.Now().Add(5 * time.Minute)),
				RefreshToken:     "new-refresh-token-abc",
				AccessExpiresIn:  durationpb.New(5 * time.Minute),
				RefreshExpiresIn: durationpb.New(24 * time.Hour),
			}, nil
		},
	}

	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create refresh request
	reqBody := map[string]string{
		"refresh_token": "refresh-token-xyz",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/auth/refresh", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "user-123", respBody["user_id"])
	assert.NotEmpty(t, respBody["access_token"])

	// Check cookies were updated
	cookies := resp.Cookies()
	var accessCookie, refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			accessCookie = c
		}
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	assert.NotNil(t, accessCookie)
	assert.NotNil(t, refreshCookie)
}

// TestRevokeHandler_Success tests successful token revocation
func TestRevokeHandler_Success(t *testing.T) {
	mockClient := &mockAuthServiceClient{
		revokeFunc: func(ctx context.Context, in *pb.RevokeRequest, opts ...grpc.CallOption) (*pb.RevokeResponse, error) {
			assert.Equal(t, "refresh-token-to-revoke", in.RefreshToken)
			assert.Equal(t, "user-123", in.UserId)
			
			return &pb.RevokeResponse{}, nil
		},
	}

	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create revoke request
	reqBody := map[string]string{
		"refresh_token": "refresh-token-to-revoke",
		"user_id":       "user-123",
	}
	reqJSON, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Make request
	resp, err := http.Post(ts.URL+"/auth/revoke", "application/json", bytes.NewBuffer(reqJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check response body
	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "Token revoked", respBody["Message"])
}

// TestProtectedRoute_WithValidToken tests accessing a protected route with a valid token
func TestProtectedRoute_WithValidToken(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a valid token that expires in the future
	validToken := generateMockJWT(time.Now().Add(5 * time.Minute))

	// Make request with Authorization header
	req, err := http.NewRequest("GET", ts.URL+"/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+validToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var respBody map[string]string
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "success", respBody["message"])
}

// TestProtectedRoute_WithExpiredToken tests accessing a protected route with an expired token
func TestProtectedRoute_WithExpiredToken(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create an expired token
	expiredToken := generateMockJWT(time.Now().Add(-1 * time.Minute))

	// Make request with expired token
	req, err := http.NewRequest("GET", ts.URL+"/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "access token expired")
}

// TestProtectedRoute_WithMissingToken tests accessing a protected route without a token
func TestProtectedRoute_WithMissingToken(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Make request without token
	resp, err := http.Get(ts.URL + "/protected")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "missing access token")
}

// TestProtectedRoute_WithInvalidToken tests accessing a protected route with a malformed token
func TestProtectedRoute_WithInvalidToken(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Make request with invalid token
	req, err := http.NewRequest("GET", ts.URL+"/protected", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "invalid access token")
}

// TestProtectedRoute_WithTokenInCookie tests accessing a protected route with token in cookie
func TestProtectedRoute_WithTokenInCookie(t *testing.T) {
	mockClient := &mockAuthServiceClient{}
	router := setupTestRouter(mockClient)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create a valid token
	validToken := generateMockJWT(time.Now().Add(5 * time.Minute))

	// Make request with token in cookie
	req, err := http.NewRequest("GET", ts.URL+"/protected", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: validToken,
	})

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
