package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paper-app/backend/internal/config"
	delivery "github.com/paper-app/backend/internal/delivery/http"
	"github.com/paper-app/backend/internal/middleware"
	"github.com/paper-app/backend/internal/repository/postgres"
	"github.com/paper-app/backend/internal/usecase"
	"github.com/paper-app/backend/pkg/arxiv"
	"github.com/paper-app/backend/pkg/pubmed"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Paper App Backend Starting...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Server configured on port %s", cfg.Server.Port)

	// Connect to PostgreSQL with retry
	var pool *pgxpool.Pool
	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var err error
		pool, err = pgxpool.New(ctx, cfg.Database.URL)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				cancel()
				log.Println("Connected to PostgreSQL")
				break
			} else {
				pool.Close()
				log.Printf("Attempt %d: Failed to ping database: %v", attempt, pingErr)
			}
		} else {
			log.Printf("Attempt %d: Failed to connect to database: %v", attempt, err)
		}
		cancel()
		if attempt == 5 {
			log.Fatalf("Could not connect to database after 5 attempts")
		}
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	defer pool.Close()

	// Initialize repositories
	userRepo := postgres.NewUserRepository(pool)
	paperRepo := postgres.NewPaperRepository(pool)
	userPaperRepo := postgres.NewUserPaperRepository(pool)
	tokenRepo := postgres.NewRefreshTokenRepository(pool)

	// Initialize external API clients
	arxivClient := arxiv.NewClient()
	pubmedClient := pubmed.NewClient()

	// Initialize usecases
	authUsecase := usecase.NewAuthUsecase(userRepo, tokenRepo, &cfg.JWT, &cfg.Google)
	paperUsecase := usecase.NewPaperUsecase(paperRepo, arxivClient, pubmedClient)
	libraryUsecase := usecase.NewLibraryUsecase(userPaperRepo, paperRepo)

	// Initialize HTTP handler and middleware
	handler := delivery.NewHandler(authUsecase, paperUsecase, libraryUsecase)
	authMiddleware := middleware.NewAuthMiddleware(authUsecase)

	// Create router
	router := delivery.NewRouter(handler, authMiddleware, cfg.CORS.AllowedOrigins)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println()
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
