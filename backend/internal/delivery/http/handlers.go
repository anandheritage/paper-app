package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/middleware"
	"github.com/paper-app/backend/internal/usecase"
)

type Handler struct {
	authUsecase    *usecase.AuthUsecase
	paperUsecase   *usecase.PaperUsecase
	libraryUsecase *usecase.LibraryUsecase
}

func NewHandler(auth *usecase.AuthUsecase, paper *usecase.PaperUsecase, library *usecase.LibraryUsecase) *Handler {
	return &Handler{
		authUsecase:    auth,
		paperUsecase:   paper,
		libraryUsecase: library,
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
	sortBy := r.URL.Query().Get("sort") // "relevance", "citations", "date"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	result, err := h.paperUsecase.SearchPapers(query, source, limit, offset, sortBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to search papers")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetPaper(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
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

// GetPaperPDFURL returns the source PDF URL for a paper.
// Per arXiv Terms of Use, we direct users to arXiv.org to retrieve e-print
// content rather than storing/serving PDFs from our servers.
func (h *Handler) ProxyPDF(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
		return
	}

	paper, err := h.paperUsecase.GetPaper(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get paper")
		return
	}
	if paper == nil || paper.PDFURL == "" {
		writeError(w, http.StatusNotFound, "PDF not found")
		return
	}

	// Build the canonical source PDF URL
	pdfURL := paper.PDFURL
	if paper.Source == "arxiv" {
		pdfURL = fmt.Sprintf("https://arxiv.org/pdf/%s", paper.ExternalID)
	}

	// Redirect to the source — legal compliance with arXiv ToU
	// "Direct users to arXiv.org to retrieve e-print content"
	http.Redirect(w, r, pdfURL, http.StatusFound)
}

func (h *Handler) GetPaperHTMLURL(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
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

	// Build candidate HTML URLs to try (in order of preference)
	var candidates []string
	switch paper.Source {
	case "arxiv":
		// Try arxiv.org/html first (official, newer), then ar5iv as fallback
		candidates = append(candidates,
			fmt.Sprintf("https://arxiv.org/html/%s", paper.ExternalID),
			fmt.Sprintf("https://ar5iv.labs.arxiv.org/html/%s", paper.ExternalID),
		)
	case "pubmed":
		if paper.Metadata != nil {
			var metadata map[string]interface{}
			if err := json.Unmarshal(paper.Metadata, &metadata); err == nil {
				if pmcID, ok := metadata["pmc_id"].(string); ok && pmcID != "" {
					candidates = append(candidates, fmt.Sprintf("https://www.ncbi.nlm.nih.gov/pmc/articles/%s/", pmcID))
				}
			}
		}
	}

	if len(candidates) == 0 {
		writeError(w, http.StatusNotFound, "HTML version not available for this paper")
		return
	}

	// Check which URL is actually accessible (HEAD request with short timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	htmlURL := ""
	for _, url := range candidates {
		resp, err := client.Head(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				htmlURL = url
				break
			}
		}
	}

	if htmlURL == "" {
		writeError(w, http.StatusNotFound, "HTML version not available for this paper")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"html_url": htmlURL,
		"source":   paper.Source,
	})
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

func (h *Handler) SaveToLibrary(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")
	paperID, err := uuid.Parse(paperIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
		return
	}

	userPaper, err := h.libraryUsecase.SavePaper(userID, paperID)
	if err == usecase.ErrPaperNotFound {
		writeError(w, http.StatusNotFound, "Paper not found")
		return
	}
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
	paperID, err := uuid.Parse(paperIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
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
	paperID, err := uuid.Parse(paperIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
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

func (h *Handler) BookmarkPaper(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paperIDStr := chi.URLParam(r, "paperId")
	paperID, err := uuid.Parse(paperIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
		return
	}

	userPaper, err := h.libraryUsecase.BookmarkPaper(userID, paperID)
	if err == usecase.ErrPaperNotFound {
		writeError(w, http.StatusNotFound, "Paper not found")
		return
	}
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
	paperID, err := uuid.Parse(paperIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid paper ID")
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
