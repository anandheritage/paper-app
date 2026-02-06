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

type PaperRepository struct {
	db *pgxpool.Pool
}

func NewPaperRepository(db *pgxpool.Pool) *PaperRepository {
	return &PaperRepository{db: db}
}

func (r *PaperRepository) Create(paper *domain.Paper) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO papers (id, external_id, source, title, abstract, authors, published_date, pdf_url, metadata, citation_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (external_id) DO UPDATE SET
			title = EXCLUDED.title,
			abstract = EXCLUDED.abstract,
			authors = EXCLUDED.authors,
			published_date = EXCLUDED.published_date,
			pdf_url = EXCLUDED.pdf_url,
			metadata = EXCLUDED.metadata,
			citation_count = CASE
				WHEN EXCLUDED.citation_count > papers.citation_count THEN EXCLUDED.citation_count
				ELSE papers.citation_count
			END
		RETURNING id
	`

	if paper.ID == uuid.Nil {
		paper.ID = uuid.New()
	}
	paper.CreatedAt = time.Now()

	err := r.db.QueryRow(ctx, query,
		paper.ID,
		paper.ExternalID,
		paper.Source,
		paper.Title,
		paper.Abstract,
		paper.Authors,
		paper.PublishedDate,
		paper.PDFURL,
		paper.Metadata,
		paper.CitationCount,
		paper.CreatedAt,
	).Scan(&paper.ID)

	return err
}

func (r *PaperRepository) GetByID(id uuid.UUID) (*domain.Paper, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, external_id, source, title, abstract, authors, published_date, pdf_url, metadata, COALESCE(citation_count, 0), created_at
		FROM papers WHERE id = $1
	`

	paper := &domain.Paper{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&paper.ID,
		&paper.ExternalID,
		&paper.Source,
		&paper.Title,
		&paper.Abstract,
		&paper.Authors,
		&paper.PublishedDate,
		&paper.PDFURL,
		&paper.Metadata,
		&paper.CitationCount,
		&paper.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return paper, nil
}

func (r *PaperRepository) GetByExternalID(externalID string) (*domain.Paper, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, external_id, source, title, abstract, authors, published_date, pdf_url, metadata, COALESCE(citation_count, 0), created_at
		FROM papers WHERE external_id = $1
	`

	paper := &domain.Paper{}
	err := r.db.QueryRow(ctx, query, externalID).Scan(
		&paper.ID,
		&paper.ExternalID,
		&paper.Source,
		&paper.Title,
		&paper.Abstract,
		&paper.Authors,
		&paper.PublishedDate,
		&paper.PDFURL,
		&paper.Metadata,
		&paper.CitationCount,
		&paper.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return paper, nil
}

func (r *PaperRepository) Search(query string, source string, limit, offset int) ([]*domain.Paper, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use PostgreSQL full-text search with tsvector
	baseQuery := `
		SELECT id, external_id, source, title, abstract, authors, published_date, pdf_url, metadata, COALESCE(citation_count, 0), created_at
		FROM papers
		WHERE ($1 = '' OR search_vector @@ plainto_tsquery('english', $1) OR title ILIKE '%' || $1 || '%')
		AND ($2 = '' OR source = $2)
		ORDER BY
			CASE WHEN $1 != '' AND search_vector @@ plainto_tsquery('english', $1)
				THEN ts_rank(search_vector, plainto_tsquery('english', $1))
				ELSE 0
			END DESC,
			citation_count DESC,
			created_at DESC
		LIMIT $3 OFFSET $4
	`

	countQuery := `
		SELECT COUNT(*)
		FROM papers
		WHERE ($1 = '' OR search_vector @@ plainto_tsquery('english', $1) OR title ILIKE '%' || $1 || '%')
		AND ($2 = '' OR source = $2)
	`

	var total int
	err := r.db.QueryRow(ctx, countQuery, query, source).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, baseQuery, query, source, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var papers []*domain.Paper
	for rows.Next() {
		paper := &domain.Paper{}
		err := rows.Scan(
			&paper.ID,
			&paper.ExternalID,
			&paper.Source,
			&paper.Title,
			&paper.Abstract,
			&paper.Authors,
			&paper.PublishedDate,
			&paper.PDFURL,
			&paper.Metadata,
			&paper.CitationCount,
			&paper.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		papers = append(papers, paper)
	}

	return papers, total, nil
}

func (r *PaperRepository) Delete(id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM papers WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
