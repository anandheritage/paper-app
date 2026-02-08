package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name,omitempty"`
	AuthProvider string     `json:"auth_provider"`
	ProviderID   string     `json:"provider_id,omitempty"`
	IsAdmin      bool       `json:"is_admin"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type UserRepository interface {
	Create(user *User) error
	GetByID(id uuid.UUID) (*User, error)
	GetByEmail(email string) (*User, error)
	GetByProviderID(provider, providerID string) (*User, error)
	Update(user *User) error
	Delete(id uuid.UUID) error
	ListAll(limit, offset int) ([]*User, int, error)
	UpdateLastLogin(id uuid.UUID) error
}

// LoginEvent tracks each login/auth event for monitoring
type LoginEvent struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	AuthMethod string    `json:"auth_method"` // email, google, token_refresh
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`

	// Joined fields (not in DB)
	UserEmail string `json:"user_email,omitempty"`
	UserName  string `json:"user_name,omitempty"`
}

type LoginEventRepository interface {
	Create(event *LoginEvent) error
	ListRecent(limit, offset int) ([]*LoginEvent, int, error)
	ListByUser(userID uuid.UUID, limit, offset int) ([]*LoginEvent, error)
	CountByMethod(since time.Time) (map[string]int, error)
	ActiveUsers(since time.Time) (int, error)
	DailyLoginCounts(days int) ([]DailyCount, error)
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// AdminStats holds platform-wide usage statistics
type AdminStats struct {
	TotalUsers       int            `json:"total_users"`
	ActiveToday      int            `json:"active_today"`
	ActiveThisWeek   int            `json:"active_this_week"`
	ActiveThisMonth  int            `json:"active_this_month"`
	TotalLogins      int            `json:"total_logins"`
	LoginsByMethod   map[string]int `json:"logins_by_method"`
	TotalPapersRead  int            `json:"total_papers_read"`
	TotalBookmarks   int            `json:"total_bookmarks"`
	TotalSaved       int            `json:"total_saved"`
	DailyLogins      []DailyCount   `json:"daily_logins"`
	NewUsersToday    int            `json:"new_users_today"`
	NewUsersThisWeek int            `json:"new_users_this_week"`
}
