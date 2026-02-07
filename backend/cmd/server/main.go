package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paper-app/backend/internal/config"
	delivery "github.com/paper-app/backend/internal/delivery/http"
	"github.com/paper-app/backend/internal/middleware"
	"github.com/paper-app/backend/internal/repository/postgres"
	"github.com/paper-app/backend/internal/usecase"
	"github.com/paper-app/backend/pkg/opensearch"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Paper App Backend Starting...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Server configured on port %s", cfg.Server.Port)

	// Connect to PostgreSQL with retry (non-fatal: server starts even if DB is unavailable)
	var pool *pgxpool.Pool
	dbConnected := false
	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var err error
		pool, err = pgxpool.New(ctx, cfg.Database.URL)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				cancel()
				log.Println("Connected to PostgreSQL")
				dbConnected = true
				break
			} else {
				pool.Close()
				pool = nil
				log.Printf("Attempt %d: Failed to ping database: %v", attempt, pingErr)
			}
		} else {
			log.Printf("Attempt %d: Failed to connect to database: %v", attempt, err)
		}
		cancel()
		if attempt == 5 {
			log.Println("WARNING: Could not connect to database after 5 attempts — starting server anyway")
			// Create pool without verifying connectivity; it will reconnect when DB is available
			pool, _ = pgxpool.New(context.Background(), cfg.Database.URL)
		} else {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	if pool != nil {
		defer pool.Close()
	}
	_ = dbConnected

	// Initialize repositories
	userRepo := postgres.NewUserRepository(pool)
	paperRepo := postgres.NewPaperRepository(pool)
	userPaperRepo := postgres.NewUserPaperRepository(pool)
	tokenRepo := postgres.NewRefreshTokenRepository(pool)

	// Initialize OpenSearch client (optional)
	var osClient *opensearch.Client
	if cfg.OpenSearch.Enabled {
		osClient = opensearch.NewClient(opensearch.Config{
			Endpoint: strings.TrimRight(cfg.OpenSearch.Endpoint, "/"),
			Index:    cfg.OpenSearch.Index,
			Username: cfg.OpenSearch.Username,
			Password: cfg.OpenSearch.Password,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := osClient.Ping(ctx); err != nil {
			log.Printf("WARNING: OpenSearch not reachable (%v) — falling back to PostgreSQL search", err)
			osClient = nil
		} else {
			log.Printf("Connected to OpenSearch at %s (index: %s)", cfg.OpenSearch.Endpoint, cfg.OpenSearch.Index)
		}
		cancel()
	} else {
		log.Println("OpenSearch not configured — using PostgreSQL for search")
	}

	// Initialize usecases
	authUsecase := usecase.NewAuthUsecase(userRepo, tokenRepo, &cfg.JWT, &cfg.Google)
	paperUsecase := usecase.NewPaperUsecase(paperRepo, osClient)
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

	// Run one-time category backfill if BACKFILL_CATEGORIES=true
	if os.Getenv("BACKFILL_CATEGORIES") == "true" {
		go func() {
			log.Println("BACKFILL: Starting category backfill in background...")
			count, err := paperRepo.BackfillCategories()
			if err != nil {
				log.Printf("BACKFILL: Failed after %d rows: %v", count, err)
			} else {
				log.Printf("BACKFILL: Complete! Updated %d rows", count)
			}
		}()
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
