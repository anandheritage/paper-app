package postgres

import (
	"context"
	"errors"
	"fmt"
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
		INSERT INTO papers (id, external_id, source, title, abstract, authors, published_date, updated_date,
			pdf_url, metadata, citation_count, primary_category, categories, doi, journal_ref, comments, license, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (external_id) DO UPDATE SET
			title = EXCLUDED.title,
			abstract = EXCLUDED.abstract,
			authors = EXCLUDED.authors,
			published_date = EXCLUDED.published_date,
			updated_date = EXCLUDED.updated_date,
			pdf_url = EXCLUDED.pdf_url,
			metadata = EXCLUDED.metadata,
			primary_category = EXCLUDED.primary_category,
			categories = EXCLUDED.categories,
			doi = COALESCE(NULLIF(EXCLUDED.doi, ''), papers.doi),
			journal_ref = COALESCE(NULLIF(EXCLUDED.journal_ref, ''), papers.journal_ref),
			comments = COALESCE(NULLIF(EXCLUDED.comments, ''), papers.comments),
			license = COALESCE(NULLIF(EXCLUDED.license, ''), papers.license),
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
		paper.ID, paper.ExternalID, paper.Source, paper.Title, paper.Abstract, paper.Authors,
		paper.PublishedDate, paper.UpdatedDate, paper.PDFURL, paper.Metadata, paper.CitationCount,
		paper.PrimaryCategory, paper.Categories, paper.DOI, paper.JournalRef, paper.Comments, paper.License,
		paper.CreatedAt,
	).Scan(&paper.ID)

	return err
}

func (r *PaperRepository) BulkUpsert(papers []*domain.Paper) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	batch := &pgx.Batch{}
	for _, p := range papers {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		batch.Queue(`
			INSERT INTO papers (id, external_id, source, title, abstract, authors, published_date, updated_date,
				pdf_url, primary_category, categories, doi, journal_ref, comments, license, citation_count, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 0, $16)
			ON CONFLICT (external_id) DO UPDATE SET
				title = EXCLUDED.title,
				abstract = EXCLUDED.abstract,
				authors = EXCLUDED.authors,
				published_date = COALESCE(EXCLUDED.published_date, papers.published_date),
				updated_date = EXCLUDED.updated_date,
				primary_category = EXCLUDED.primary_category,
				categories = EXCLUDED.categories,
				doi = COALESCE(NULLIF(EXCLUDED.doi, ''), papers.doi),
				journal_ref = COALESCE(NULLIF(EXCLUDED.journal_ref, ''), papers.journal_ref),
				comments = COALESCE(NULLIF(EXCLUDED.comments, ''), papers.comments),
				license = COALESCE(NULLIF(EXCLUDED.license, ''), papers.license)
		`,
			p.ID, p.ExternalID, p.Source, p.Title, p.Abstract, p.Authors,
			p.PublishedDate, p.UpdatedDate, p.PDFURL, p.PrimaryCategory,
			p.Categories, p.DOI, p.JournalRef, p.Comments, p.License, p.CreatedAt,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	inserted := 0
	for range papers {
		ct, err := br.Exec()
		if err != nil {
			continue
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}
	return inserted, nil
}

func (r *PaperRepository) GetByID(id uuid.UUID) (*domain.Paper, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, external_id, source, title, abstract, authors, published_date, updated_date,
			pdf_url, metadata, COALESCE(citation_count, 0),
			COALESCE(primary_category, ''), categories,
			COALESCE(doi, ''), COALESCE(journal_ref, ''), COALESCE(comments, ''), COALESCE(license, ''),
			created_at
		FROM papers WHERE id = $1
	`

	paper := &domain.Paper{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&paper.ID, &paper.ExternalID, &paper.Source, &paper.Title, &paper.Abstract, &paper.Authors,
		&paper.PublishedDate, &paper.UpdatedDate, &paper.PDFURL, &paper.Metadata, &paper.CitationCount,
		&paper.PrimaryCategory, &paper.Categories,
		&paper.DOI, &paper.JournalRef, &paper.Comments, &paper.License,
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
		SELECT id, external_id, source, title, abstract, authors, published_date, updated_date,
			pdf_url, metadata, COALESCE(citation_count, 0),
			COALESCE(primary_category, ''), categories,
			COALESCE(doi, ''), COALESCE(journal_ref, ''), COALESCE(comments, ''), COALESCE(license, ''),
			created_at
		FROM papers WHERE external_id = $1
	`

	paper := &domain.Paper{}
	err := r.db.QueryRow(ctx, query, externalID).Scan(
		&paper.ID, &paper.ExternalID, &paper.Source, &paper.Title, &paper.Abstract, &paper.Authors,
		&paper.PublishedDate, &paper.UpdatedDate, &paper.PDFURL, &paper.Metadata, &paper.CitationCount,
		&paper.PrimaryCategory, &paper.Categories,
		&paper.DOI, &paper.JournalRef, &paper.Comments, &paper.License,
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

func (r *PaperRepository) Search(query string, source string, limit, offset int, sortBy string) ([]*domain.Paper, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if sortBy == "" {
		sortBy = "relevance"
	}

	whereClause := `
		WHERE ($1 = '' OR search_vector @@ plainto_tsquery('english', $1) OR title ILIKE '%' || $1 || '%')
		AND ($2 = '' OR source = $2)
	`

	var orderClause string
	switch sortBy {
	case "citations":
		orderClause = `
			ORDER BY citation_count DESC,
				CASE WHEN $1 != '' AND search_vector @@ plainto_tsquery('english', $1)
					THEN ts_rank(search_vector, plainto_tsquery('english', $1))
					ELSE 0
				END DESC,
				published_date DESC NULLS LAST
		`
	case "date":
		orderClause = `
			ORDER BY published_date DESC NULLS LAST,
				CASE WHEN $1 != '' AND search_vector @@ plainto_tsquery('english', $1)
					THEN ts_rank(search_vector, plainto_tsquery('english', $1))
					ELSE 0
				END DESC
		`
	default:
		orderClause = `
			ORDER BY
				CASE WHEN $1 != '' AND search_vector @@ plainto_tsquery('english', $1)
					THEN ts_rank(search_vector, plainto_tsquery('english', $1))
					ELSE 0
				END DESC,
				citation_count DESC,
				published_date DESC NULLS LAST
		`
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, external_id, source, title, abstract, authors, published_date, updated_date,
			pdf_url, metadata, COALESCE(citation_count, 0),
			COALESCE(primary_category, ''), categories,
			COALESCE(doi, ''), COALESCE(journal_ref, ''), COALESCE(comments, ''), COALESCE(license, ''),
			created_at
		FROM papers %s %s LIMIT $3 OFFSET $4
	`, whereClause, orderClause)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM papers %s`, whereClause)

	var total int
	err := r.db.QueryRow(ctx, countQuery, query, source).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, selectQuery, query, source, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var papers []*domain.Paper
	for rows.Next() {
		paper := &domain.Paper{}
		err := rows.Scan(
			&paper.ID, &paper.ExternalID, &paper.Source, &paper.Title, &paper.Abstract, &paper.Authors,
			&paper.PublishedDate, &paper.UpdatedDate, &paper.PDFURL, &paper.Metadata, &paper.CitationCount,
			&paper.PrimaryCategory, &paper.Categories,
			&paper.DOI, &paper.JournalRef, &paper.Comments, &paper.License,
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

	_, err := r.db.Exec(ctx, `DELETE FROM papers WHERE id = $1`, id)
	return err
}

// CountByCategory returns the number of papers per primary_category.
func (r *PaperRepository) CountByCategory() ([]domain.CategoryCount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(primary_category, 'unknown'), COUNT(*)
		FROM papers
		WHERE primary_category IS NOT NULL AND primary_category != ''
		GROUP BY primary_category
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []domain.CategoryCount
	for rows.Next() {
		var cc domain.CategoryCount
		if err := rows.Scan(&cc.Category, &cc.Count); err != nil {
			return nil, err
		}
		counts = append(counts, cc)
	}
	return counts, nil
}

// StreamAll iterates over all papers in batches and calls fn for each batch.
func (r *PaperRepository) StreamAll(ctx context.Context, batchSize int, fn func(papers []*domain.Paper) error) error {
	offset := 0
	for {
		rows, err := r.db.Query(ctx, `
			SELECT id, external_id, source, title, abstract, authors, published_date, updated_date,
				pdf_url, metadata, COALESCE(citation_count, 0),
				COALESCE(primary_category, ''), categories,
				COALESCE(doi, ''), COALESCE(journal_ref, ''), COALESCE(comments, ''), COALESCE(license, ''),
				created_at
			FROM papers
			WHERE title IS NOT NULL AND title != ''
			ORDER BY external_id
			LIMIT $1 OFFSET $2
		`, batchSize, offset)
		if err != nil {
			return err
		}

		var papers []*domain.Paper
		for rows.Next() {
			paper := &domain.Paper{}
			err := rows.Scan(
				&paper.ID, &paper.ExternalID, &paper.Source, &paper.Title, &paper.Abstract, &paper.Authors,
				&paper.PublishedDate, &paper.UpdatedDate, &paper.PDFURL, &paper.Metadata, &paper.CitationCount,
				&paper.PrimaryCategory, &paper.Categories,
				&paper.DOI, &paper.JournalRef, &paper.Comments, &paper.License,
				&paper.CreatedAt,
			)
			if err != nil {
				rows.Close()
				return err
			}
			papers = append(papers, paper)
		}
		rows.Close()

		if len(papers) == 0 {
			break
		}

		if err := fn(papers); err != nil {
			return err
		}

		offset += batchSize
	}
	return nil
}
