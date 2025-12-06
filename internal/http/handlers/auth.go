package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	pb "github.com/andro-kes/auth_service/proto"
)

type AuthManager struct {
	Client pb.AuthServiceClient
}

func NewAuthManager(client pb.AuthServiceClient) *AuthManager {
	return &AuthManager{
		Client: client,
	}
}

func (am *AuthManager) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req pb.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	resp, err := am.Client.Login(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if resp.RefreshToken != "" {
		setRefreshTokenInCookie(w, r, resp)
	}

	if resp.AccessToken != "" {
		setAccessTokenInCookie(w, r, resp)
	}

	out := map[string]any{
		"user_id": resp.UserId,
	}
	if resp.AccessToken != "" {
		out["access_token"] = resp.AccessToken
	}
	if resp.AccessExpiresIn != nil {
		out["access_expires_in_seconds"] = int64(resp.AccessExpiresIn.AsDuration().Seconds())
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (am *AuthManager) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req pb.RegisterRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := am.Client.Register(r.Context(), &req)
	if err != nil {
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	out := map[string]any{
		"user_id": resp.UserId,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (am *AuthManager) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req pb.RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Failed to decode requets body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := am.Client.Refresh(r.Context(), &req)
	if err != nil {
		http.Error(w, "Failed to refresh token", http.StatusInternalServerError)
		return
	}

	if resp.RefreshToken != "" {
		setRefreshTokenInCookie(w, r, resp)
	}

	if resp.AccessToken != "" {
		setAccessTokenInCookie(w, r, resp)
	}

	out := map[string]any{
		"user_id": resp.UserId,
	}
	if resp.AccessToken != "" {
		out["access_token"] = resp.AccessToken
	}
	if resp.AccessExpiresIn != nil {
		out["access_expires_in_seconds"] = int64(resp.AccessExpiresIn.AsDuration().Seconds())
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func setRefreshTokenInCookie(w http.ResponseWriter, r *http.Request, resp *pb.TokenResponse) {
	c := &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
	if resp.RefreshExpiresIn != nil {
		c.Expires = time.Now().Add(resp.RefreshExpiresIn.AsDuration())
	}
	http.SetCookie(w, c)
}

func setAccessTokenInCookie(w http.ResponseWriter, r *http.Request, resp *pb.TokenResponse) {
	ac := &http.Cookie{
		Name:     "access_token",
		Value:    resp.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
	if resp.AccessExpiresIn != nil {
		ac.Expires = time.Now().Add(resp.AccessExpiresIn.AsDuration())
	} else {
		ac.Expires = time.Now().Add(5 * time.Minute)
	}
	http.SetCookie(w, ac)

	w.Header().Set("Authorization", "Bearer "+resp.AccessToken)
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
}
