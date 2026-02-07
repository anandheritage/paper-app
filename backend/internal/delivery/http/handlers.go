package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/internal/middleware"
	"github.com/paper-app/backend/internal/usecase"
)

type Handler struct {
	authUsecase    *usecase.AuthUsecase
	paperUsecase   *usecase.PaperUsecase
	libraryUsecase *usecase.LibraryUsecase
	userRepo       domain.UserRepository
}

func NewHandler(auth *usecase.AuthUsecase, paper *usecase.PaperUsecase, library *usecase.LibraryUsecase, userRepo domain.UserRepository) *Handler {
	return &Handler{
		authUsecase:    auth,
		paperUsecase:   paper,
		libraryUsecase: library,
		userRepo:       userRepo,
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// Auth handlers

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type authResponse struct {
	User   interface{} `json:"user"`
	Tokens interface{} `json:"tokens"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	user, tokens, err := h.authUsecase.Register(req.Email, req.Password, req.Name)
	if err == usecase.ErrEmailExists {
		writeError(w, http.StatusConflict, "Email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{User: user, Tokens: tokens})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, tokens, err := h.authUsecase.Login(req.Email, req.Password)
	if err == usecase.ErrInvalidCredentials {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to login")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{User: user, Tokens: tokens})
}

type googleLoginRequest struct {
	Code        string `json:"code"`
	AccessToken string `json:"access_token"` // legacy: implicit flow (deprecated)
}

func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req googleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" && req.AccessToken == "" {
		writeError(w, http.StatusBadRequest, "Authorization code is required")
		return
	}

	user, tokens, err := h.authUsecase.GoogleLogin(req.Code, req.AccessToken)
	if err == usecase.ErrInvalidGoogleToken {
		writeError(w, http.StatusUnauthorized, "Invalid Google token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to authenticate with Google")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{User: user, Tokens: tokens})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tokens, err := h.authUsecase.RefreshToken(req.RefreshToken)
	if err == usecase.ErrInvalidToken || err == usecase.ErrTokenExpired {
		writeError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.authUsecase.Logout(req.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.authUsecase.GetUserByID(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// Paper handlers

func (h *Handler) SearchPapers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	source := r.URL.Query().Get("source")
	sortBy := r.URL.Query().Get("sort")         // "relevance", "citations", "date"
	catFilter := r.URL.Query().Get("categories") // comma-separated: "Computer Science,Mathematics"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	categories := usecase.ParseCategories(catFilter)

	result, err := h.paperUsecase.SearchPapers(query, source, limit, offset, sortBy, categories)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to search papers")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetCategories returns all categories with paper counts.
func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.paperUsecase.GetCategories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get categories")
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

// GetGroupedCategories returns categories organized by group.
func (h *Handler) GetGroupedCategories(w http.ResponseWriter, r *http.Request) {
	grouped, err := h.paperUsecase.GetGroupedCategories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get categories")
		return
	}
	writeJSON(w, http.StatusOK, grouped)
}

// GetPaper returns a paper by ID. Tries OpenSearch first (corpusid), then PostgreSQL (UUID).
func (h *Handler) GetPaper(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	// Try OpenSearch first (primary source for S2 data)
	doc, err := h.paperUsecase.GetPaperFromOS(idStr)
	if err == nil && doc != nil {
		writeJSON(w, http.StatusOK, doc)
		return
	}

	// Fallback: try PostgreSQL by UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paper not found")
		return
	}

	paper, err := h.paperUsecase.GetPaper(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get paper")
		return
	}
	if paper == nil {
		writeError(w, http.StatusNotFound, "Paper not found")
		return
	}

	writeJSON(w, http.StatusOK, paper)
}

// Library handlers

func (h *Handler) GetLibrary(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.libraryUsecase.GetLibrary(userID, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get library")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SaveToLibrary saves a paper to the user's library.
// Accepts either a PG UUID or an OpenSearch corpusid/arXiv ID.
func (h *Handler) SaveToLibrary(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")

	// Resolve the paper ID to a PG UUID (auto-creates PG record if needed)
	paperID, err := h.paperUsecase.EnsurePaperInDB(paperIDStr)
	if err != nil {
		if err == usecase.ErrPaperNotFound || err == usecase.ErrPaperNotFoundOS {
			writeError(w, http.StatusNotFound, "Paper not found")
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to save paper")
		}
		return
	}

	userPaper, err := h.libraryUsecase.SavePaper(userID, paperID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save paper")
		return
	}

	writeJSON(w, http.StatusCreated, userPaper)
}

func (h *Handler) RemoveFromLibrary(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")

	// Resolve ID
	paperID, err := h.paperUsecase.EnsurePaperInDB(paperIDStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paper not in library")
		return
	}

	err = h.libraryUsecase.RemovePaper(userID, paperID)
	if err == usecase.ErrPaperNotInLibrary {
		writeError(w, http.StatusNotFound, "Paper not in library")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to remove paper")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateLibraryPaper(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")
	paperID, err := h.paperUsecase.EnsurePaperInDB(paperIDStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paper not found")
		return
	}

	var input usecase.UpdatePaperInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userPaper, err := h.libraryUsecase.UpdatePaper(userID, paperID, &input)
	if err == usecase.ErrPaperNotInLibrary {
		writeError(w, http.StatusNotFound, "Paper not in library")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update paper")
		return
	}

	writeJSON(w, http.StatusOK, userPaper)
}

// Bookmark handlers

func (h *Handler) GetBookmarks(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.libraryUsecase.GetBookmarks(userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get bookmarks")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// BookmarkPaper bookmarks a paper for the user.
// Accepts either a PG UUID or an OpenSearch corpusid/arXiv ID.
func (h *Handler) BookmarkPaper(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")

	// Resolve the paper ID to a PG UUID
	paperID, err := h.paperUsecase.EnsurePaperInDB(paperIDStr)
	if err != nil {
		if err == usecase.ErrPaperNotFound || err == usecase.ErrPaperNotFoundOS {
			writeError(w, http.StatusNotFound, "Paper not found")
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to bookmark paper")
		}
		return
	}

	userPaper, err := h.libraryUsecase.BookmarkPaper(userID, paperID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to bookmark paper")
		return
	}

	writeJSON(w, http.StatusCreated, userPaper)
}

func (h *Handler) UnbookmarkPaper(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")
	paperID, err := h.paperUsecase.EnsurePaperInDB(paperIDStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "Paper not in library")
		return
	}

	err = h.libraryUsecase.UnbookmarkPaper(userID, paperID)
	if err == usecase.ErrPaperNotInLibrary {
		writeError(w, http.StatusNotFound, "Paper not in library")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to unbookmark paper")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Discover handler

func (h *Handler) GetDiscover(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	seed := r.URL.Query().Get("seed")
	if seed == "" {
		// Default: date + user ID for daily consistency per user
		seed = time.Now().Format("2006-01-02") + userID.String()
	}

	categories, _ := h.libraryUsecase.GetUserCategories(userID)
	excludeIDs, _ := h.libraryUsecase.GetUserPaperExternalIDs(userID)

	result, err := h.paperUsecase.Discover(categories, excludeIDs, seed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get suggestions")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Admin handlers

type adminUserResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	AuthProvider string `json:"auth_provider"`
	IsAdmin      bool   `json:"is_admin"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toAdminUser(u *domain.User) adminUserResponse {
	return adminUserResponse{
		ID:           u.ID.String(),
		Email:        u.Email,
		Name:         u.Name,
		AuthProvider: u.AuthProvider,
		IsAdmin:      u.IsAdmin,
		CreatedAt:    u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    u.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *Handler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 50
	}

	users, total, err := h.userRepo.ListAll(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	var resp []adminUserResponse
	for _, u := range users {
		resp = append(resp, toAdminUser(u))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users":  resp,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	_, total, err := h.userRepo.ListAll(1, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_users": total,
	})
}
