package usecase

import (
	"github.com/google/uuid"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/pkg/arxiv"
	"github.com/paper-app/backend/pkg/pubmed"
)

type PaperUsecase struct {
	paperRepo domain.PaperRepository
	arxiv     *arxiv.Client
	pubmed    *pubmed.Client
}

func NewPaperUsecase(paperRepo domain.PaperRepository, arxivClient *arxiv.Client, pubmedClient *pubmed.Client) *PaperUsecase {
	return &PaperUsecase{
		paperRepo: paperRepo,
		arxiv:     arxivClient,
		pubmed:    pubmedClient,
	}
}

type SearchResult struct {
	Papers []*domain.Paper `json:"papers"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
}

func (u *PaperUsecase) SearchPapers(query, source string, limit, offset int) (*SearchResult, error) {
	var papers []*domain.Paper
	var total int

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

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
		// Search both sources
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
