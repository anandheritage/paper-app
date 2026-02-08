package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paper-app/backend/internal/domain"
)

type LoginEventRepository struct {
	db *pgxpool.Pool
}

func NewLoginEventRepository(db *pgxpool.Pool) *LoginEventRepository {
	return &LoginEventRepository{db: db}
}

func (r *LoginEventRepository) Create(event *domain.LoginEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	event.CreatedAt = time.Now()

	query := `
		INSERT INTO login_events (id, user_id, auth_method, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query,
		event.ID, event.UserID, event.AuthMethod, event.IPAddress, event.UserAgent, event.CreatedAt,
	)
	return err
}

func (r *LoginEventRepository) ListRecent(limit, offset int) ([]*domain.LoginEvent, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM login_events`).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT le.id, le.user_id, le.auth_method, le.ip_address, le.user_agent, le.created_at,
		       COALESCE(u.email, '') AS user_email, COALESCE(u.name, '') AS user_name
		FROM login_events le
		LEFT JOIN users u ON u.id = le.user_id
		ORDER BY le.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*domain.LoginEvent
	for rows.Next() {
		e := &domain.LoginEvent{}
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AuthMethod, &e.IPAddress, &e.UserAgent, &e.CreatedAt,
			&e.UserEmail, &e.UserName,
		); err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, nil
}

func (r *LoginEventRepository) ListByUser(userID uuid.UUID, limit, offset int) ([]*domain.LoginEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, user_id, auth_method, ip_address, user_agent, created_at
		FROM login_events
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.LoginEvent
	for rows.Next() {
		e := &domain.LoginEvent{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.AuthMethod, &e.IPAddress, &e.UserAgent, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *LoginEventRepository) CountByMethod(since time.Time) (map[string]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT auth_method, COUNT(*) FROM login_events
		WHERE created_at >= $1
		GROUP BY auth_method
	`
	rows, err := r.db.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var method string
		var count int
		if err := rows.Scan(&method, &count); err != nil {
			return nil, err
		}
		result[method] = count
	}
	return result, nil
}

func (r *LoginEventRepository) ActiveUsers(since time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM login_events WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}

func (r *LoginEventRepository) DailyLoginCounts(days int) ([]domain.DailyCount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT DATE(created_at) AS day, COUNT(*) AS cnt
		FROM login_events
		WHERE created_at >= NOW() - ($1 || ' days')::INTERVAL
		GROUP BY day
		ORDER BY day ASC
	`
	rows, err := r.db.Query(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []domain.DailyCount
	for rows.Next() {
		var dc domain.DailyCount
		var t time.Time
		if err := rows.Scan(&t, &dc.Count); err != nil {
			return nil, err
		}
		dc.Date = t.Format("2006-01-02")
		counts = append(counts, dc)
	}
	return counts, nil
}
