package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/paper-app/backend/internal/middleware"
)

func NewRouter(handler *Handler, authMiddleware *middleware.AuthMiddleware, allowedOrigins []string) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", handler.Register)
			r.Post("/login", handler.Login)
			r.Post("/google", handler.GoogleLogin)
			r.Post("/refresh", handler.RefreshToken)
			r.Post("/logout", handler.Logout)

			// Protected auth routes
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.Authenticate)
				r.Get("/me", handler.GetCurrentUser)
			})
		})

		// Paper routes (public search, protected for actions)
		r.Route("/papers", func(r chi.Router) {
			r.Get("/search", handler.SearchPapers)
			r.Get("/categories", handler.GetCategories)
			r.Get("/categories/grouped", handler.GetGroupedCategories)
			r.Get("/{id}", handler.GetPaper)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			// Library routes
			r.Route("/library", func(r chi.Router) {
				r.Get("/", handler.GetLibrary)
				r.Post("/{paperId}", handler.SaveToLibrary)
				r.Delete("/{paperId}", handler.RemoveFromLibrary)
				r.Patch("/{paperId}", handler.UpdateLibraryPaper)
			})

			// Bookmark routes
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", handler.GetBookmarks)
				r.Post("/{paperId}", handler.BookmarkPaper)
				r.Delete("/{paperId}", handler.UnbookmarkPaper)
			})
		})
	})

	return r
}
