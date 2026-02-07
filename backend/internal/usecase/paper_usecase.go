package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/pkg/opensearch"
)

var ErrPaperNotFoundOS = errors.New("paper not found in search index")

type PaperUsecase struct {
	paperRepo domain.PaperRepository // PG — only used for library operations
	osClient  *opensearch.Client     // OpenSearch — primary source for search + detail
}

func NewPaperUsecase(paperRepo domain.PaperRepository, osClient *opensearch.Client) *PaperUsecase {
	return &PaperUsecase{
		paperRepo: paperRepo,
		osClient:  osClient,
	}
}

// ---------- Search ----------

// SearchResult is the API response for paper search.
type SearchResult struct {
	Papers []*opensearch.PaperDoc `json:"papers"`
	Total  int                    `json:"total"`
	Offset int                    `json:"offset"`
	Limit  int                    `json:"limit"`
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

	// Use OpenSearch as the primary search engine
	if u.osClient != nil {
		return u.searchOpenSearch(query, categories, limit, offset, sort)
	}

	// Fallback to PostgreSQL search (legacy)
	papers, total, err := u.paperRepo.Search(query, source, limit, offset, sort)
	if err != nil {
		return nil, err
	}

	// Convert domain.Paper to opensearch.PaperDoc for consistent API response
	docs := make([]*opensearch.PaperDoc, 0, len(papers))
	for _, p := range papers {
		docs = append(docs, domainPaperToDoc(p))
	}

	return &SearchResult{
		Papers: docs,
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
		log.Printf("OpenSearch search failed: %v", err)

		// Fallback to PostgreSQL if available
		if u.paperRepo != nil {
			papers, total, pgErr := u.paperRepo.Search(query, "", limit, offset, sort)
			if pgErr != nil {
				return nil, pgErr
			}
			docs := make([]*opensearch.PaperDoc, 0, len(papers))
			for _, p := range papers {
				docs = append(docs, domainPaperToDoc(p))
			}
			return &SearchResult{Papers: docs, Total: total, Offset: offset, Limit: limit}, nil
		}
		return nil, err
	}

	// Extract PaperDocs from hits
	papers := make([]*opensearch.PaperDoc, 0, len(osResult.Hits))
	for _, hit := range osResult.Hits {
		doc := hit.Doc
		papers = append(papers, &doc)
	}

	return &SearchResult{
		Papers: papers,
		Total:  osResult.Total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// ---------- Paper Detail ----------

// GetPaperFromOS retrieves a paper by its S2 corpusid or external ID from OpenSearch.
func (u *PaperUsecase) GetPaperFromOS(id string) (*opensearch.PaperDoc, error) {
	if u.osClient == nil {
		return nil, ErrPaperNotFoundOS
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try direct lookup by _id (corpusid)
	doc, err := u.osClient.GetByID(ctx, id)
	if err == nil && doc != nil {
		return doc, nil
	}

	// Try by external_id (arXiv ID)
	doc, err = u.osClient.SearchByExternalID(ctx, id)
	if err == nil && doc != nil {
		return doc, nil
	}

	return nil, ErrPaperNotFoundOS
}

// GetPaper retrieves a paper by UUID from PostgreSQL (legacy, for library).
func (u *PaperUsecase) GetPaper(id uuid.UUID) (*domain.Paper, error) {
	if u.paperRepo == nil {
		return nil, ErrPaperNotFoundOS
	}
	return u.paperRepo.GetByID(id)
}

// GetPaperByExternalID retrieves a paper by external ID from PostgreSQL.
func (u *PaperUsecase) GetPaperByExternalID(externalID string) (*domain.Paper, error) {
	if u.paperRepo == nil {
		return nil, ErrPaperNotFoundOS
	}
	return u.paperRepo.GetByExternalID(externalID)
}

// ---------- Library Support ----------

// EnsurePaperInDB makes sure a paper exists in PostgreSQL (for library operations).
// If the paper is not in PG, fetches it from OpenSearch and creates a record.
// Returns the PG UUID for the paper.
func (u *PaperUsecase) EnsurePaperInDB(idStr string) (uuid.UUID, error) {
	// Try parsing as UUID first (existing PG paper)
	if pgID, err := uuid.Parse(idStr); err == nil {
		if u.paperRepo != nil {
			paper, err := u.paperRepo.GetByID(pgID)
			if err == nil && paper != nil {
				return paper.ID, nil
			}
		}
	}

	// Try as external_id in PG
	if u.paperRepo != nil {
		paper, err := u.paperRepo.GetByExternalID(idStr)
		if err == nil && paper != nil {
			return paper.ID, nil
		}
	}

	// Not in PG — fetch from OpenSearch and create a PG record
	if u.osClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Try by _id first (corpusid)
		doc, err := u.osClient.GetByID(ctx, idStr)
		if err != nil || doc == nil {
			// Try by external_id
			doc, err = u.osClient.SearchByExternalID(ctx, idStr)
			if err != nil || doc == nil {
				return uuid.Nil, ErrPaperNotFound
			}
		}

		// Create PG record from OS data
		newPaper := osPaperDocToDomain(doc)
		if u.paperRepo != nil {
			if err := u.paperRepo.Create(newPaper); err != nil {
				// If create fails (e.g., duplicate), try to find existing
				existing, findErr := u.paperRepo.GetByExternalID(doc.ExternalID)
				if findErr == nil && existing != nil {
					return existing.ID, nil
				}
				return uuid.Nil, err
			}
		}
		return newPaper.ID, nil
	}

	return uuid.Nil, ErrPaperNotFound
}

// ---------- Discover ----------

// DiscoverResult is the response for the discover/suggestion endpoint.
type DiscoverResult struct {
	PaperOfTheDay *opensearch.PaperDoc   `json:"paper_of_the_day"`
	Suggestions   []*opensearch.PaperDoc `json:"suggestions"`
	Categories    []string               `json:"based_on_categories"`
	TopCited      []*opensearch.PaperDoc `json:"top_cited,omitempty"`
}

// Discover returns random paper suggestions based on user interest categories.
// Uses a seed for deterministic randomness (same result within a seed value, e.g. daily).
func (u *PaperUsecase) Discover(categories []string, excludeExternalIDs []string, seed string) (*DiscoverResult, error) {
	if u.osClient == nil {
		return &DiscoverResult{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	papers, err := u.osClient.GetRandomPapers(ctx, categories, excludeExternalIDs, seed, 6)
	if err != nil || len(papers) == 0 {
		if err != nil {
			log.Printf("Discover search failed: %v", err)
		}
		// Try without categories as fallback (popular random papers)
		papers, err = u.osClient.GetRandomPapers(ctx, nil, excludeExternalIDs, seed, 6)
		if err != nil {
			return nil, err
		}
	}

	result := &DiscoverResult{
		Categories: categories,
	}

	if len(papers) > 0 {
		result.PaperOfTheDay = papers[0]
		if len(papers) > 1 {
			result.Suggestions = papers[1:]
		}
	}

	// Fetch top-cited papers of all time from diverse fields
	topCited, err := u.osClient.GetTopCitedDiverseFields(ctx, 5)
	if err != nil {
		log.Printf("Failed to fetch top-cited papers: %v", err)
		// Non-fatal — the section simply won't appear
	} else {
		result.TopCited = topCited
	}

	return result, nil
}

// ---------- Categories ----------

// GetCategories returns category info with paper counts.
func (u *PaperUsecase) GetCategories() ([]domain.CategoryInfo, error) {
	var counts map[string]int64

	if u.osClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		counts, err = u.osClient.GetCategoryCounts(ctx)
		if err != nil {
			log.Printf("OpenSearch category counts failed: %v", err)
		}
	}

	// Fallback to PostgreSQL
	if counts == nil && u.paperRepo != nil {
		pgCounts, err := u.paperRepo.CountByCategory()
		if err != nil {
			return nil, err
		}
		counts = make(map[string]int64)
		for _, c := range pgCounts {
			counts[c.Category] = c.Count
		}
	}

	if counts == nil {
		return nil, nil
	}

	// Build CategoryInfo — for S2 data, categories are human-readable already
	var categories []domain.CategoryInfo
	for catID, count := range counts {
		if count < 10 { // Skip very low-count categories
			continue
		}
		// S2 categories are already human-readable (e.g., "Computer Science", "Mathematics")
		categories = append(categories, domain.CategoryInfo{
			ID:    catID,
			Name:  catID, // S2 categories are already readable
			Group: catID, // Each S2 field is its own group
			Count: count,
		})
	}

	// Sort by count descending
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

// BackfillCategories populates primary_category and categories columns from metadata JSON.
func (u *PaperUsecase) BackfillCategories() (int64, error) {
	if u.paperRepo == nil {
		return 0, nil
	}
	return u.paperRepo.BackfillCategories()
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

// ---------- Converters ----------

// domainPaperToDoc converts a PG domain.Paper to an opensearch.PaperDoc for API responses.
func domainPaperToDoc(p *domain.Paper) *opensearch.PaperDoc {
	var pubDate *string
	if p.PublishedDate != nil {
		s := p.PublishedDate.Format("2006-01-02")
		pubDate = &s
	}

	return &opensearch.PaperDoc{
		ID:              p.ID.String(),
		ExternalID:      p.ExternalID,
		Source:          p.Source,
		Title:           p.Title,
		Abstract:        p.Abstract,
		Authors:         json.RawMessage(p.Authors),
		PublishedDate:   pubDate,
		PDFURL:          p.PDFURL,
		PrimaryCategory: p.PrimaryCategory,
		Categories:      p.Categories,
		DOI:             p.DOI,
		JournalRef:      p.JournalRef,
		CitationCount:   p.CitationCount,
	}
}

// osPaperDocToDomain converts an OS PaperDoc to a PG domain.Paper.
func osPaperDocToDomain(doc *opensearch.PaperDoc) *domain.Paper {
	authorsJSON, _ := json.Marshal(doc.Authors)

	var pubDate *time.Time
	if doc.PublishedDate != nil {
		// Try multiple formats
		for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
			if t, err := time.Parse(layout, *doc.PublishedDate); err == nil {
				pubDate = &t
				break
			}
		}
	}

	return &domain.Paper{
		ID:              uuid.New(),
		ExternalID:      doc.ExternalID,
		Source:          doc.Source,
		Title:           doc.Title,
		Abstract:        doc.Abstract,
		Authors:         authorsJSON,
		PublishedDate:   pubDate,
		PDFURL:          doc.PDFURL,
		PrimaryCategory: doc.PrimaryCategory,
		Categories:      doc.Categories,
		DOI:             doc.DOI,
		JournalRef:      doc.JournalRef,
		CitationCount:   doc.CitationCount,
		CreatedAt:       time.Now(),
	}
}
