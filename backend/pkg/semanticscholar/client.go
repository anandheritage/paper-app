package semanticscholar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paper-app/backend/internal/domain"
)

const apiBaseURL = "https://api.semanticscholar.org/graph/v1"

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type SearchResult struct {
	Papers       []*domain.Paper
	TotalResults int
}

// API response types
type searchResponse struct {
	Total  int           `json:"total"`
	Offset int           `json:"offset"`
	Data   []paperResult `json:"data"`
}

type paperResult struct {
	PaperID       string        `json:"paperId"`
	Title         string        `json:"title"`
	Abstract      string        `json:"abstract"`
	Year          int           `json:"year"`
	CitationCount int           `json:"citationCount"`
	URL           string        `json:"url"`
	Authors       []authorInfo  `json:"authors"`
	ExternalIDs   externalIDs   `json:"externalIds"`
	OpenAccessPDF *openAccessPDF `json:"openAccessPdf"`
	PublicationDate string      `json:"publicationDate"` // "YYYY-MM-DD"
}

type authorInfo struct {
	AuthorID string `json:"authorId"`
	Name     string `json:"name"`
}

type externalIDs struct {
	ArXiv  string `json:"ArXiv"`
	DOI    string `json:"DOI"`
	PubMed string `json:"PubMed"`
	PMCID  string `json:"PMCID,omitempty"`
}

type openAccessPDF struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

// Search searches Semantic Scholar for papers. sortBy can be "relevance", "citationCount", or "publicationDate".
func (c *Client) Search(query string, limit, offset int, sortBy string) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("fields", "title,abstract,year,citationCount,url,authors,externalIds,openAccessPdf,publicationDate")

	// Semantic Scholar API supports sorting
	if sortBy == "citationCount" {
		params.Set("sort", "citationCount:desc")
	} else if sortBy == "publicationDate" {
		params.Set("sort", "publicationDate:desc")
	}
	// Default (relevance) = no sort param needed

	reqURL := fmt.Sprintf("%s/paper/search?%s", apiBaseURL, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "PaperApp/1.0 (academic-reader)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("semantic scholar API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("semantic scholar API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResp searchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	papers := make([]*domain.Paper, 0, len(searchResp.Data))
	for _, result := range searchResp.Data {
		paper := resultToPaper(&result)
		if paper != nil {
			papers = append(papers, paper)
		}
	}

	return &SearchResult{
		Papers:       papers,
		TotalResults: searchResp.Total,
	}, nil
}

func resultToPaper(r *paperResult) *domain.Paper {
	if r.Title == "" {
		return nil
	}

	// Determine source and external ID
	source := "semanticscholar"
	externalID := r.PaperID
	if r.ExternalIDs.ArXiv != "" {
		source = "arxiv"
		externalID = r.ExternalIDs.ArXiv
	} else if r.ExternalIDs.PubMed != "" {
		source = "pubmed"
		externalID = r.ExternalIDs.PubMed
	}

	// Build authors
	authors := make([]domain.Author, 0, len(r.Authors))
	for _, a := range r.Authors {
		if a.Name != "" {
			authors = append(authors, domain.Author{Name: strings.TrimSpace(a.Name)})
		}
	}
	authorsJSON, _ := json.Marshal(authors)

	// Parse published date
	var publishedDate *time.Time
	if r.PublicationDate != "" {
		if t, err := time.Parse("2006-01-02", r.PublicationDate); err == nil {
			publishedDate = &t
		}
	} else if r.Year > 0 {
		t := time.Date(r.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		publishedDate = &t
	}

	// Build PDF URL
	pdfURL := ""
	if r.OpenAccessPDF != nil && r.OpenAccessPDF.URL != "" {
		pdfURL = r.OpenAccessPDF.URL
	} else if r.ExternalIDs.ArXiv != "" {
		pdfURL = fmt.Sprintf("https://arxiv.org/pdf/%s", r.ExternalIDs.ArXiv)
	} else if r.ExternalIDs.DOI != "" {
		pdfURL = fmt.Sprintf("https://doi.org/%s", r.ExternalIDs.DOI)
	}

	// Build metadata
	metadata := map[string]interface{}{
		"citation_count": r.CitationCount,
		"s2_url":         r.URL,
	}
	if r.ExternalIDs.DOI != "" {
		metadata["doi"] = r.ExternalIDs.DOI
	}
	if r.ExternalIDs.ArXiv != "" {
		metadata["html_url"] = fmt.Sprintf("https://arxiv.org/html/%s", r.ExternalIDs.ArXiv)
	}
	if r.ExternalIDs.PMCID != "" {
		metadata["pmc_id"] = r.ExternalIDs.PMCID
		metadata["html_url"] = fmt.Sprintf("https://www.ncbi.nlm.nih.gov/pmc/articles/%s/", r.ExternalIDs.PMCID)
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &domain.Paper{
		ExternalID:    externalID,
		Source:        source,
		Title:         strings.TrimSpace(r.Title),
		Abstract:      strings.TrimSpace(r.Abstract),
		Authors:       authorsJSON,
		PublishedDate: publishedDate,
		PDFURL:        pdfURL,
		Metadata:      metadataJSON,
	}
}
