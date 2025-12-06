package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

// PropagateAuthToGRPC extracts the access token from Authorization header or
// access_token cookie, checks expiry (quick decode of JWT payload only),
// returns 401 if missing/expired (so frontend can call /auth/refresh), and
// otherwise injects the Authorization value into outgoing gRPC metadata.
func PropagateAuthToGRPC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		if auth == "" {
			c, err := r.Cookie("access_token")
			if err == nil && c.Value != "" {
				auth = "Bearer " + c.Value
			}
		}

		if auth == "" {
			http.Error(w, "missing access token", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
			http.Error(w, "invalid access token", http.StatusUnauthorized)
			return
		}

		raw := strings.TrimSpace(auth[len(prefix):])
		if raw == "" {
			http.Error(w, "empty access token", http.StatusUnauthorized)
			return
		}

		// Quick expiry check without signature verification:
		expired, err := tokenExpired(raw)
		if err != nil {
			// malformed token: force refresh / re-login
			http.Error(w, "invalid access token", http.StatusUnauthorized)
			return
		}
		if expired {
			http.Error(w, "access token expired", http.StatusUnauthorized)
			return
		}

		// token not expired — inject into outgoing gRPC metadata
		ctx := metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", auth))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// tokenExpired decodes JWT payload and returns true if exp <= now.
func tokenExpired(token string) (bool, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return false, errors.New("malformed token")
	}
	payload := parts[1]
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// try standard base64 if padding present
		raw, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return false, err
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return false, err
	}

	v, ok := claims["exp"]
	if !ok {
		return false, errors.New("exp not present")
	}
	var expInt int64
	switch t := v.(type) {
	case float64:
		expInt = int64(t)
	case json.Number:
		i, err := t.Int64()
		if err != nil {
			return false, err
		}
		expInt = i
	default:
		return false, errors.New("invalid exp type")
	}
	now := time.Now().Unix()
	return now >= expInt, nil
}
