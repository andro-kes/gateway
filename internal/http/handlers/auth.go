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

	if resp.AccessToken != "" {
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
	json.NewEncoder(w).Encode(out)
}

func (am *AuthManager) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req pb.RegisterRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "failed to decode request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := am.Client.Register(r.Context(), &req)
	if err != nil {
		http.Error(w, "failed to register user", http.StatusInternalServerError)
		return
	}

	out := map[string]string{
		"user_id": resp.UserId,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}