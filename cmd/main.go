package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/andro-kes/auth_service/proto"
	"github.com/andro-kes/gateway/internal/http/handlers"
	"github.com/andro-kes/gateway/internal/logger"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	zl := logger.Logger()
	defer zl.Sync()

	var (
		httpAddr = flag.String("http", os.Getenv("HTTP_ADDR"), "HTTP address to listen on")
		grpcAddr = flag.String("grpc", os.Getenv("GRPC_ADDR"), "gRPC address to listen on")
	)
	flag.Parse()

	conn, err := grpc.NewClient(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	authClient := pb.NewAuthServiceClient(conn)
	authManager := handlers.NewAuthManager(authClient)

	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authManager.LoginHandler)
		// r.Post("/register", authManager.RegisterHandler) // Removed: handler not defined
		r.Post("/refresh", authManager.RefreshHandler)
		// r.Post("/revoke", authManager.RevokeHandler) // Removed: handler not defined
	})

	server := http.Server{
		Addr: *httpAddr,
		Handler: r,
	}

	svrError := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			svrError <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <- svrError:
		zl.Warn("Failed to start HTTP server", zap.Error(err))
		panic(err.Error())
	case <- shutdown:
		zl.Info("System shutdown")
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
        panic(err.Error())
	}
}
