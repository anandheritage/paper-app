package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Paper struct {
	ID            uuid.UUID       `json:"id"`
	ExternalID    string          `json:"external_id"`
	Source        string          `json:"source"`
	Title         string          `json:"title"`
	Abstract      string          `json:"abstract,omitempty"`
	Authors       json.RawMessage `json:"authors,omitempty"`
	PublishedDate *time.Time      `json:"published_date,omitempty"`
	PDFURL        string          `json:"pdf_url,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CitationCount int             `json:"citation_count"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Author struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation,omitempty"`
}

type PaperRepository interface {
	Create(paper *Paper) error
	GetByID(id uuid.UUID) (*Paper, error)
	GetByExternalID(externalID string) (*Paper, error)
	Search(query string, source string, limit, offset int) ([]*Paper, int, error)
	Delete(id uuid.UUID) error
}

type PaperSearchParams struct {
	Query     string
	Source    string
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
}
