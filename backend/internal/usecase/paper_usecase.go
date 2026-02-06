package usecase

import (
	"log"

	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/pkg/arxiv"
	"github.com/paper-app/backend/pkg/openalex"
	"github.com/paper-app/backend/pkg/pubmed"
)

type PaperUsecase struct {
	paperRepo domain.PaperRepository
	arxiv     *arxiv.Client
	pubmed    *pubmed.Client
	openalex  *openalex.Client
}

func NewPaperUsecase(paperRepo domain.PaperRepository, arxivClient *arxiv.Client, pubmedClient *pubmed.Client, openalexClient *openalex.Client) *PaperUsecase {
	return &PaperUsecase{
		paperRepo: paperRepo,
		arxiv:     arxivClient,
		pubmed:    pubmedClient,
		openalex:  openalexClient,
	}
}

type SearchResult struct {
	Papers []*domain.Paper `json:"papers"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

func (u *PaperUsecase) SearchPapers(query, source string, limit, offset int, sort string) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if sort == "" {
		sort = "relevance"
	}

	// For PubMed-specific searches, use the PubMed API (we don't have PubMed
	// bulk data locally).
	if source == "pubmed" {
		return u.searchPubMed(query, limit, offset)
	}

	// For all/arXiv searches, prefer local DB (contains ~2.4M arXiv papers
	// from the Kaggle bulk import).
	papers, total, err := u.paperRepo.Search(query, source, limit, offset, sort)
	if err != nil {
		log.Printf("Local DB search failed: %v — falling back to APIs", err)
		return u.searchAPIs(query, source, sort, limit, offset)
	}

	if total > 0 {
		return &SearchResult{
			Papers: papers,
			Total:  total,
			Offset: offset,
			Limit:  limit,
		}, nil
	}

	// No local results — fall back to external APIs
	log.Printf("No local results for %q — falling back to APIs", query)
	return u.searchAPIs(query, source, sort, limit, offset)
}

// searchAPIs tries OpenAlex first, then arXiv+PubMed as a last resort.
func (u *PaperUsecase) searchAPIs(query, source, sort string, limit, offset int) (*SearchResult, error) {
	// Try OpenAlex (best quality, has citation counts)
	if u.openalex != nil {
		results, err := u.openalex.Search(query, source, sort, limit, offset)
		if err == nil && len(results.Papers) > 0 {
			// Cache in local DB
			for _, p := range results.Papers {
				u.paperRepo.Create(p)
			}
			return &SearchResult{
				Papers: results.Papers,
				Total:  results.TotalResults,
				Offset: offset,
				Limit:  limit,
			}, nil
		}
		if err != nil {
			log.Printf("OpenAlex search failed: %v", err)
		}
	}

	// Fall back to individual source APIs
	return u.fallbackSearch(query, source, limit, offset)
}

// searchPubMed searches the PubMed API directly.
func (u *PaperUsecase) searchPubMed(query string, limit, offset int) (*SearchResult, error) {
	results, err := u.pubmed.Search(query, limit, offset)
	if err != nil {
		return nil, err
	}
	for _, p := range results.Papers {
		u.paperRepo.Create(p)
	}
	return &SearchResult{
		Papers: results.Papers,
		Total:  results.TotalResults,
		Offset: offset,
		Limit:  limit,
	}, nil
}

// fallbackSearch uses arXiv and/or PubMed APIs directly when OpenAlex is unavailable.
func (u *PaperUsecase) fallbackSearch(query, source string, limit, offset int) (*SearchResult, error) {
	var papers []*domain.Paper
	var total int

	switch source {
	case "arxiv":
		results, err := u.arxiv.Search(query, limit, offset)
		if err != nil {
			return nil, err
		}
		papers = results.Papers
		total = results.TotalResults
	case "pubmed":
		results, err := u.pubmed.Search(query, limit, offset)
		if err != nil {
			return nil, err
		}
		papers = results.Papers
		total = results.TotalResults
	default:
		// Search both sources, combine results
		arxivResults, arxivErr := u.arxiv.Search(query, limit/2, offset/2)
		pubmedResults, pubmedErr := u.pubmed.Search(query, limit/2, offset/2)

		if arxivErr == nil {
			papers = append(papers, arxivResults.Papers...)
			total += arxivResults.TotalResults
		}
		if pubmedErr == nil {
			papers = append(papers, pubmedResults.Papers...)
			total += pubmedResults.TotalResults
		}

		if arxivErr != nil && pubmedErr != nil {
			return nil, arxivErr
		}
	}

	// Cache papers in database
	for _, paper := range papers {
		u.paperRepo.Create(paper)
	}

	return &SearchResult{
		Papers: papers,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}, nil
}

func (u *PaperUsecase) GetPaper(id uuid.UUID) (*domain.Paper, error) {
	return u.paperRepo.GetByID(id)
}

func (u *PaperUsecase) GetPaperByExternalID(externalID string) (*domain.Paper, error) {
	return u.paperRepo.GetByExternalID(externalID)
}

func (u *PaperUsecase) GetOrFetchPaper(externalID, source string) (*domain.Paper, error) {
	paper, err := u.paperRepo.GetByExternalID(externalID)
	if err != nil {
		return nil, err
	}
	if paper != nil {
		return paper, nil
	}

	// Fetch from source
	switch source {
	case "arxiv":
		paper, err = u.arxiv.GetPaper(externalID)
	case "pubmed":
		paper, err = u.pubmed.GetPaper(externalID)
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	if paper == nil {
		return nil, nil
	}

	// Cache in database
	if err := u.paperRepo.Create(paper); err != nil {
		return nil, err
	}

	return paper, nil
}
