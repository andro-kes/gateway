package handlers

import (
	"net/http"

	"google.golang.org/grpc/metadata"
)

func PropagateAuthToGRPC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		if auth == "" {
			if c, err := r.Cookie("access_token"); err == nil && c.Value != "" {
				auth = "Bearer " + c.Value
			}
		}

		if auth == "" {
			http.Error(w, "Access token was expired", http.StatusUnauthorized)
			return
		}

		prefix := "Bearer "
		if len(auth) <= len(prefix) {
			http.Error(w, "Invalid access token", http.StatusUnauthorized)
			return
		}

		ctx := metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", auth))
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}