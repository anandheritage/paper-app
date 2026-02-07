package usecase

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/pkg/opensearch"
)

type PaperUsecase struct {
	paperRepo domain.PaperRepository
	osClient  *opensearch.Client // nil if OpenSearch is not configured
}

func NewPaperUsecase(paperRepo domain.PaperRepository, osClient *opensearch.Client) *PaperUsecase {
	return &PaperUsecase{
		paperRepo: paperRepo,
		osClient:  osClient,
	}
}

type SearchResult struct {
	Papers []*domain.Paper `json:"papers"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

func (u *PaperUsecase) SearchPapers(query, source string, limit, offset int, sort string, categories []string) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if sort == "" {
		sort = "relevance"
	}

	// Use OpenSearch if available
	if u.osClient != nil {
		return u.searchOpenSearch(query, categories, limit, offset, sort)
	}

	// Fallback to PostgreSQL search
	papers, total, err := u.paperRepo.Search(query, source, limit, offset, sort)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Papers: papers,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

func (u *PaperUsecase) searchOpenSearch(query string, categories []string, limit, offset int, sort string) (*SearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	osResult, err := u.osClient.Search(ctx, opensearch.SearchParams{
		Query:      query,
		Categories: categories,
		SortBy:     sort,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		log.Printf("OpenSearch search failed, falling back to PG: %v", err)
		// Fallback to PostgreSQL
		papers, total, pgErr := u.paperRepo.Search(query, "", limit, offset, sort)
		if pgErr != nil {
			return nil, pgErr
		}
		return &SearchResult{Papers: papers, Total: total, Offset: offset, Limit: limit}, nil
	}

	// Convert OpenSearch hits to domain Papers
	papers := make([]*domain.Paper, 0, len(osResult.Hits))
	for _, hit := range osResult.Hits {
		paper := convertOSHitToPaper(hit)
		papers = append(papers, paper)
	}

	return &SearchResult{
		Papers: papers,
		Total:  osResult.Total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

func convertOSHitToPaper(hit *opensearch.SearchHit) *domain.Paper {
	doc := hit.Doc

	id, _ := uuid.Parse(doc.ID)

	authorsJSON, _ := json.Marshal(doc.Authors)

	var pubDate *time.Time
	if doc.PublishedDate != nil {
		if t, err := time.Parse("2006-01-02", *doc.PublishedDate); err == nil {
			pubDate = &t
		}
	}

	var updDate *time.Time
	if doc.UpdatedDate != nil {
		if t, err := time.Parse("2006-01-02", *doc.UpdatedDate); err == nil {
			updDate = &t
		}
	}

	return &domain.Paper{
		ID:              id,
		ExternalID:      doc.ExternalID,
		Source:          doc.Source,
		Title:           doc.Title,
		Abstract:        doc.Abstract,
		Authors:         authorsJSON,
		PublishedDate:   pubDate,
		UpdatedDate:     updDate,
		PDFURL:          doc.PDFURL,
		CitationCount:   doc.CitationCount,
		PrimaryCategory: doc.PrimaryCategory,
		Categories:      doc.Categories,
		DOI:             doc.DOI,
		JournalRef:      doc.JournalRef,
		Comments:        doc.Comments,
		License:         doc.License,
	}
}

func (u *PaperUsecase) GetPaper(id uuid.UUID) (*domain.Paper, error) {
	return u.paperRepo.GetByID(id)
}

func (u *PaperUsecase) GetPaperByExternalID(externalID string) (*domain.Paper, error) {
	return u.paperRepo.GetByExternalID(externalID)
}

// GetCategories returns category info with paper counts.
func (u *PaperUsecase) GetCategories() ([]domain.CategoryInfo, error) {
	var counts map[string]int64

	if u.osClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		counts, err = u.osClient.GetCategoryCounts(ctx)
		if err != nil {
			log.Printf("OpenSearch category counts failed, falling back to PG: %v", err)
		}
	}

	// Fallback to PostgreSQL if OpenSearch failed or unavailable
	if counts == nil {
		pgCounts, err := u.paperRepo.CountByCategory()
		if err != nil {
			return nil, err
		}
		counts = make(map[string]int64)
		for _, c := range pgCounts {
			counts[c.Category] = c.Count
		}
	}

	// Build CategoryInfo with human-readable names
	var categories []domain.CategoryInfo
	for catID, count := range counts {
		if count < 10 { // Skip very low-count categories
			continue
		}
		info := domain.GetCategoryInfo(catID)
		info.Count = count
		categories = append(categories, info)
	}

	// Sort by count descending (simple bubble sort for small slice)
	for i := 0; i < len(categories); i++ {
		for j := i + 1; j < len(categories); j++ {
			if categories[j].Count > categories[i].Count {
				categories[i], categories[j] = categories[j], categories[i]
			}
		}
	}

	return categories, nil
}

// GetGroupedCategories returns categories organized by group.
func (u *PaperUsecase) GetGroupedCategories() (map[string][]domain.CategoryInfo, error) {
	categories, err := u.GetCategories()
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]domain.CategoryInfo)
	for _, cat := range categories {
		group := cat.Group
		if group == "" {
			group = "Other"
		}
		grouped[group] = append(grouped[group], cat)
	}

	return grouped, nil
}

// ParseCategories extracts category IDs from a comma-separated string.
func ParseCategories(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var categories []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			categories = append(categories, p)
		}
	}
	return categories
}
