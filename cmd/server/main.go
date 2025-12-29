package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pbAuth "github.com/andro-kes/auth_service/proto"
	"github.com/andro-kes/gateway/internal/http/handlers"
	"github.com/andro-kes/gateway/internal/logger"
	pbInv "github.com/andro-kes/inventory_service/proto"
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

	authClient := pbAuth.NewAuthServiceClient(conn)
	authManager := handlers.NewAuthManager(authClient)

	invClient := pbInv.NewInventoryServiceClient(conn)
	invManager := handlers.NewInvManager(invClient)

	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", authManager.LoginHandler)
		r.Post("/register", authManager.RegisterHandler)
		r.Post("/refresh", authManager.RefreshHandler)
		r.Post("/revoke", authManager.RevokeHandler)
		r.Get("/health", handlers.CheckHealth)
	})

	r.Route("/inventory", func(r chi.Router) {
		r.Post("/create", invManager.CreateHandler)
		r.Post("/delete", invManager.DeleteHandler)
		r.Post("/get", invManager.GetHandler)
		r.Post("/list", invManager.ListHandler)
		r.Post("/update", invManager.UpdateHandler)
	})

	server := http.Server{
		Addr:    *httpAddr,
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
	case err := <-svrError:
		zl.Warn("Failed to start HTTP server", zap.Error(err))
		panic(err.Error())
	case <-shutdown:
		zl.Info("System shutdown")
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		panic(err.Error())
	}
}
