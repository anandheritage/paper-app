package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paper-app/backend/internal/domain"
)

type UserPaperRepository struct {
	db *pgxpool.Pool
}

func NewUserPaperRepository(db *pgxpool.Pool) *UserPaperRepository {
	return &UserPaperRepository{db: db}
}

func (r *UserPaperRepository) Create(userPaper *domain.UserPaper) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO user_papers (id, user_id, paper_id, status, is_bookmarked, reading_progress, notes, tags, saved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, paper_id) DO UPDATE SET
			status = EXCLUDED.status,
			is_bookmarked = EXCLUDED.is_bookmarked,
			reading_progress = EXCLUDED.reading_progress,
			notes = EXCLUDED.notes,
			tags = EXCLUDED.tags
		RETURNING id
	`

	if userPaper.ID == uuid.Nil {
		userPaper.ID = uuid.New()
	}
	userPaper.SavedAt = time.Now()

	err := r.db.QueryRow(ctx, query,
		userPaper.ID,
		userPaper.UserID,
		userPaper.PaperID,
		userPaper.Status,
		userPaper.IsBookmarked,
		userPaper.ReadingProgress,
		userPaper.Notes,
		userPaper.Tags,
		userPaper.SavedAt,
	).Scan(&userPaper.ID)

	return err
}

func (r *UserPaperRepository) GetByUserAndPaper(userID, paperID uuid.UUID) (*domain.UserPaper, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT up.id, up.user_id, up.paper_id, up.status, up.is_bookmarked, up.reading_progress,
			   up.notes, up.tags, up.saved_at, up.last_read_at,
			   p.id, p.external_id, p.source, p.title, p.abstract, p.authors, p.published_date, p.pdf_url, p.metadata, p.created_at
		FROM user_papers up
		JOIN papers p ON up.paper_id = p.id
		WHERE up.user_id = $1 AND up.paper_id = $2
	`

	userPaper := &domain.UserPaper{Paper: &domain.Paper{}}
	err := r.db.QueryRow(ctx, query, userID, paperID).Scan(
		&userPaper.ID,
		&userPaper.UserID,
		&userPaper.PaperID,
		&userPaper.Status,
		&userPaper.IsBookmarked,
		&userPaper.ReadingProgress,
		&userPaper.Notes,
		&userPaper.Tags,
		&userPaper.SavedAt,
		&userPaper.LastReadAt,
		&userPaper.Paper.ID,
		&userPaper.Paper.ExternalID,
		&userPaper.Paper.Source,
		&userPaper.Paper.Title,
		&userPaper.Paper.Abstract,
		&userPaper.Paper.Authors,
		&userPaper.Paper.PublishedDate,
		&userPaper.Paper.PDFURL,
		&userPaper.Paper.Metadata,
		&userPaper.Paper.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return userPaper, nil
}

func (r *UserPaperRepository) GetByUser(userID uuid.UUID, status string, bookmarked *bool, limit, offset int) ([]*domain.UserPaper, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	baseQuery := `
		SELECT up.id, up.user_id, up.paper_id, up.status, up.is_bookmarked, up.reading_progress,
			   up.notes, up.tags, up.saved_at, up.last_read_at,
			   p.id, p.external_id, p.source, p.title, p.abstract, p.authors, p.published_date, p.pdf_url, p.metadata, p.created_at
		FROM user_papers up
		JOIN papers p ON up.paper_id = p.id
		WHERE up.user_id = $1
		AND ($2 = '' OR up.status = $2)
		AND ($3::boolean IS NULL OR up.is_bookmarked = $3)
		ORDER BY up.saved_at DESC
		LIMIT $4 OFFSET $5
	`

	countQuery := `
		SELECT COUNT(*)
		FROM user_papers up
		WHERE up.user_id = $1
		AND ($2 = '' OR up.status = $2)
		AND ($3::boolean IS NULL OR up.is_bookmarked = $3)
	`

	var total int
	err := r.db.QueryRow(ctx, countQuery, userID, status, bookmarked).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, baseQuery, userID, status, bookmarked, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var userPapers []*domain.UserPaper
	for rows.Next() {
		userPaper := &domain.UserPaper{Paper: &domain.Paper{}}
		err := rows.Scan(
			&userPaper.ID,
			&userPaper.UserID,
			&userPaper.PaperID,
			&userPaper.Status,
			&userPaper.IsBookmarked,
			&userPaper.ReadingProgress,
			&userPaper.Notes,
			&userPaper.Tags,
			&userPaper.SavedAt,
			&userPaper.LastReadAt,
			&userPaper.Paper.ID,
			&userPaper.Paper.ExternalID,
			&userPaper.Paper.Source,
			&userPaper.Paper.Title,
			&userPaper.Paper.Abstract,
			&userPaper.Paper.Authors,
			&userPaper.Paper.PublishedDate,
			&userPaper.Paper.PDFURL,
			&userPaper.Paper.Metadata,
			&userPaper.Paper.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		userPapers = append(userPapers, userPaper)
	}

	return userPapers, total, nil
}

func (r *UserPaperRepository) Update(userPaper *domain.UserPaper) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE user_papers
		SET status = $3, is_bookmarked = $4, reading_progress = $5, notes = $6, tags = $7, last_read_at = $8
		WHERE user_id = $1 AND paper_id = $2
	`

	_, err := r.db.Exec(ctx, query,
		userPaper.UserID,
		userPaper.PaperID,
		userPaper.Status,
		userPaper.IsBookmarked,
		userPaper.ReadingProgress,
		userPaper.Notes,
		userPaper.Tags,
		userPaper.LastReadAt,
	)
	return err
}

func (r *UserPaperRepository) Delete(userID, paperID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM user_papers WHERE user_id = $1 AND paper_id = $2`
	_, err := r.db.Exec(ctx, query, userID, paperID)
	return err
}

func (r *UserPaperRepository) EnforceReadingLimit(userID uuid.UUID, maxReading int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE user_papers SET status = 'saved'
		WHERE user_id = $1 AND id IN (
			SELECT id FROM user_papers
			WHERE user_id = $1 AND status = 'reading'
			ORDER BY COALESCE(last_read_at, saved_at) DESC
			OFFSET $2
		)
	`
	_, err := r.db.Exec(ctx, query, userID, maxReading)
	return err
}

func (r *UserPaperRepository) GetUserCategories(userID uuid.UUID) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT DISTINCT cat
		FROM user_papers up
		JOIN papers p ON up.paper_id = p.id
		CROSS JOIN LATERAL unnest(p.categories) AS cat
		WHERE up.user_id = $1
		  AND p.categories IS NOT NULL
		LIMIT 20
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

func (r *UserPaperRepository) GetUserPaperExternalIDs(userID uuid.UUID) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT p.external_id
		FROM user_papers up
		JOIN papers p ON up.paper_id = p.id
		WHERE up.user_id = $1
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
