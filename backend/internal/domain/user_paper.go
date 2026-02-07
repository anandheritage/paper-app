package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type UserPaper struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	PaperID         uuid.UUID       `json:"paper_id"`
	Status          string          `json:"status"`
	IsBookmarked    bool            `json:"is_bookmarked"`
	ReadingProgress int             `json:"reading_progress"`
	Notes           string          `json:"notes,omitempty"`
	Tags            json.RawMessage `json:"tags,omitempty"`
	SavedAt         time.Time       `json:"saved_at"`
	LastReadAt      *time.Time      `json:"last_read_at,omitempty"`
	BookmarkedAt    *time.Time      `json:"bookmarked_at,omitempty"`
	Paper           *Paper          `json:"paper,omitempty"`
}

type ReadingSession struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PaperID   uuid.UUID  `json:"paper_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	PagesRead int        `json:"pages_read"`
}

type UserPaperRepository interface {
	Create(userPaper *UserPaper) error
	GetByUserAndPaper(userID, paperID uuid.UUID) (*UserPaper, error)
	GetByUser(userID uuid.UUID, status string, bookmarked *bool, limit, offset int) ([]*UserPaper, int, error)
	Update(userPaper *UserPaper) error
	Delete(userID, paperID uuid.UUID) error
	EnforceReadingLimit(userID uuid.UUID, maxReading int) error
	GetUserCategories(userID uuid.UUID) ([]string, error)
	GetUserPaperExternalIDs(userID uuid.UUID) ([]string, error)
}

type ReadingSessionRepository interface {
	Create(session *ReadingSession) error
	Update(session *ReadingSession) error
	GetByUser(userID uuid.UUID, limit, offset int) ([]*ReadingSession, error)
}

const (
	StatusSaved    = "saved"
	StatusReading  = "reading"
	StatusFinished = "finished"
)
