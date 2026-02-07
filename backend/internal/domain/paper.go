package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Paper struct {
	ID              uuid.UUID       `json:"id"`
	ExternalID      string          `json:"external_id"`
	Source          string          `json:"source"`
	Title           string          `json:"title"`
	Abstract        string          `json:"abstract,omitempty"`
	Authors         json.RawMessage `json:"authors,omitempty"`
	PublishedDate   *time.Time      `json:"published_date,omitempty"`
	UpdatedDate     *time.Time      `json:"updated_date,omitempty"`
	PDFURL          string          `json:"pdf_url,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CitationCount   int             `json:"citation_count"`
	PrimaryCategory string          `json:"primary_category,omitempty"`
	Categories      []string        `json:"categories,omitempty"`
	DOI             string          `json:"doi,omitempty"`
	JournalRef      string          `json:"journal_ref,omitempty"`
	Comments        string          `json:"comments,omitempty"`
	License         string          `json:"license,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type Author struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation,omitempty"`
}

// PaperRepository handles paper CRUD in PostgreSQL (source of truth).
type PaperRepository interface {
	Create(paper *Paper) error
	BulkUpsert(papers []*Paper) (int, error)
	GetByID(id uuid.UUID) (*Paper, error)
	GetByExternalID(externalID string) (*Paper, error)
	Search(query string, source string, limit, offset int, sortBy string) ([]*Paper, int, error)
	Delete(id uuid.UUID) error
	CountByCategory() ([]CategoryCount, error)
	StreamAll(ctx context.Context, batchSize int, fn func(papers []*Paper) error) error
}

// PaperSearcher handles search operations (OpenSearch).
// If OpenSearch is not configured, the system falls back to PaperRepository.Search().
type PaperSearcher interface {
	Search(ctx context.Context, params SearchParams) (*SearchResult, error)
	Index(ctx context.Context, paper *Paper) error
	BulkIndex(ctx context.Context, papers []*Paper) error
	DeleteIndex(ctx context.Context) error
	CreateIndex(ctx context.Context) error
	GetCategoryCounts(ctx context.Context) ([]CategoryCount, error)
}

type SearchParams struct {
	Query      string
	Categories []string // filter by categories (e.g., ["cs.AI", "cs.LG"])
	SortBy     string   // "relevance", "citations", "date"
	Limit      int
	Offset     int
}

type SearchResult struct {
	Papers []*Paper `json:"papers"`
	Total  int      `json:"total"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// CategoryInfo provides human-readable category information.
type CategoryInfo struct {
	ID    string `json:"id"`    // e.g., "cs.AI"
	Name  string `json:"name"`  // e.g., "Artificial Intelligence"
	Group string `json:"group"` // e.g., "Computer Science"
	Count int64  `json:"count"` // number of papers
}

type PaperSearchParams struct {
	Query     string
	Source    string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}
