package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RefreshTokenRepository interface {
	Create(token *RefreshToken) error
	GetByTokenHash(tokenHash string) (*RefreshToken, error)
	DeleteByUserID(userID uuid.UUID) error
	DeleteByTokenHash(tokenHash string) error
	DeleteExpired() error
}
