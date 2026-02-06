package openalex

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

const baseURL = "https://api.openalex.org"

// arXiv source ID in OpenAlex
const arxivSourceID = "S4306400806"

// Client is an OpenAlex API client for academic paper search.
// OpenAlex is free, has no rate limits (with polite pool), and provides citation counts.
type Client struct {
	httpClient *http.Client
	email      string // for polite pool — faster responses
}

// NewClient creates a new OpenAlex API client.
// email is optional but recommended — it puts you in the "polite pool" for faster responses.
func NewClient(email string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		email:      email,
	}
}

// SearchResult holds the search response
type SearchResult struct {
	Papers       []*domain.Paper
	TotalResults int
}

// --- OpenAlex API response types ---

type searchResponse struct {
	Meta struct {
		Count   int `json:"count"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
	Results []workResult `json:"results"`
}

type workResult struct {
	ID                    string                  `json:"id"`
	DOI                   string                  `json:"doi"`
	Title                 string                  `json:"title"`
	DisplayName           string                  `json:"display_name"`
	PublicationYear       int                     `json:"publication_year"`
	PublicationDate       string                  `json:"publication_date"`
	Type                  string                  `json:"type"`
	CitedByCount          int                     `json:"cited_by_count"`
	Authorships           []authorship            `json:"authorships"`
	PrimaryLocation       *location               `json:"primary_location"`
	OpenAccess            *openAccess             `json:"open_access"`
	IDs                   map[string]interface{}  `json:"ids"`
	AbstractInvertedIndex map[string][]int        `json:"abstract_inverted_index"`
}

type authorship struct {
	AuthorPosition string `json:"author_position"`
	Author         struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Orcid       string `json:"orcid"`
	} `json:"author"`
	Institutions []struct {
		DisplayName string `json:"display_name"`
	} `json:"institutions"`
}

type location struct {
	IsOA           bool    `json:"is_oa"`
	LandingPageURL string  `json:"landing_page_url"`
	PDFURL         string  `json:"pdf_url"`
	Source         *source `json:"source"`
}

type source struct {
	ID                   string `json:"id"`
	DisplayName          string `json:"display_name"`
	HostOrganizationName string `json:"host_organization_name"`
	Type                 string `json:"type"`
}

type openAccess struct {
	IsOA     bool   `json:"is_oa"`
	OAStatus string `json:"oa_status"`
	OAURL    string `json:"oa_url"`
}

// Search queries OpenAlex for papers.
// source can be "" (all), "arxiv", or "pubmed".
// sortBy can be "relevance", "citations", or "date".
func (c *Client) Search(query, sourceFilter, sortBy string, limit, offset int) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// OpenAlex uses page-based pagination
	page := (offset / limit) + 1
	if page < 1 {
		page = 1
	}

	params := url.Values{}
	params.Set("search", query)
	params.Set("per_page", fmt.Sprintf("%d", limit))
	params.Set("page", fmt.Sprintf("%d", page))

	// Sort
	switch sortBy {
	case "citations":
		params.Set("sort", "cited_by_count:desc")
	case "date":
		params.Set("sort", "publication_date:desc")
	default:
		// Default: relevance (no sort param needed when using search)
	}

	// Source filter
	switch sourceFilter {
	case "arxiv":
		params.Set("filter", "primary_location.source.id:"+arxivSourceID)
	}
	// Note: PubMed-specific searches go through PubMed API directly

	// Polite pool — OpenAlex recommends providing email for faster responses
	if c.email != "" {
		params.Set("mailto", c.email)
	}

	reqURL := fmt.Sprintf("%s/works?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	ua := "PaperApp/1.0 (academic-reader)"
	if c.email != "" {
		ua = fmt.Sprintf("PaperApp/1.0 (mailto:%s)", c.email)
	}
	req.Header.Set("User-Agent", ua)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAlex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAlex API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResp searchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	papers := make([]*domain.Paper, 0, len(searchResp.Results))
	for i := range searchResp.Results {
		paper := workToPaper(&searchResp.Results[i])
		if paper != nil {
			papers = append(papers, paper)
		}
	}

	return &SearchResult{
		Papers:       papers,
		TotalResults: searchResp.Meta.Count,
	}, nil
}

// workToPaper converts an OpenAlex work result to our domain Paper model
func workToPaper(w *workResult) *domain.Paper {
	title := w.Title
	if title == "" {
		title = w.DisplayName
	}
	if title == "" {
		return nil
	}

	// Determine source and external ID
	paperSource := "openalex"
	externalID := strings.TrimPrefix(w.ID, "https://openalex.org/")

	// Check for arXiv
	arxivID := extractArXivID(w)
	if arxivID != "" {
		paperSource = "arxiv"
		externalID = arxivID
	} else {
		// Check for PubMed
		pmid := extractPMID(w)
		if pmid != "" {
			paperSource = "pubmed"
			externalID = pmid
		}
	}

	// Build authors
	authors := make([]domain.Author, 0, len(w.Authorships))
	for _, a := range w.Authorships {
		if a.Author.DisplayName != "" {
			author := domain.Author{Name: strings.TrimSpace(a.Author.DisplayName)}
			if len(a.Institutions) > 0 && a.Institutions[0].DisplayName != "" {
				author.Affiliation = a.Institutions[0].DisplayName
			}
			authors = append(authors, author)
		}
	}
	authorsJSON, _ := json.Marshal(authors)

	// Parse date
	var publishedDate *time.Time
	if w.PublicationDate != "" {
		if t, err := time.Parse("2006-01-02", w.PublicationDate); err == nil {
			publishedDate = &t
		}
	}
	if publishedDate == nil && w.PublicationYear > 0 {
		t := time.Date(w.PublicationYear, 1, 1, 0, 0, 0, 0, time.UTC)
		publishedDate = &t
	}

	// Build PDF URL
	pdfURL := ""
	if w.PrimaryLocation != nil && w.PrimaryLocation.PDFURL != "" {
		pdfURL = w.PrimaryLocation.PDFURL
	} else if w.OpenAccess != nil && w.OpenAccess.OAURL != "" {
		pdfURL = w.OpenAccess.OAURL
	}
	// Fallback for arXiv papers
	if pdfURL == "" && arxivID != "" {
		pdfURL = fmt.Sprintf("https://arxiv.org/pdf/%s", arxivID)
	}
	// Fallback to DOI link
	if pdfURL == "" && w.DOI != "" {
		pdfURL = w.DOI // DOI URL often resolves to publisher page
	}

	// Reconstruct abstract from inverted index
	abstract := reconstructAbstract(w.AbstractInvertedIndex)

	// Build metadata
	metadata := map[string]interface{}{
		"citation_count": w.CitedByCount,
		"openalex_id":    w.ID,
		"type":           w.Type,
	}
	if w.DOI != "" {
		metadata["doi"] = w.DOI
	}
	if arxivID != "" {
		metadata["html_url"] = fmt.Sprintf("https://arxiv.org/html/%s", arxivID)
	}
	if pmcid := extractPMCID(w); pmcid != "" {
		metadata["pmc_id"] = pmcid
		metadata["html_url"] = fmt.Sprintf("https://www.ncbi.nlm.nih.gov/pmc/articles/%s/", pmcid)
	}
	if w.PrimaryLocation != nil && w.PrimaryLocation.Source != nil {
		metadata["venue"] = w.PrimaryLocation.Source.DisplayName
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &domain.Paper{
		ExternalID:    externalID,
		Source:        paperSource,
		Title:         strings.TrimSpace(title),
		Abstract:      strings.TrimSpace(abstract),
		Authors:       authorsJSON,
		PublishedDate: publishedDate,
		PDFURL:        pdfURL,
		Metadata:      metadataJSON,
		CitationCount: w.CitedByCount,
	}
}

// extractArXivID tries to extract an arXiv ID from an OpenAlex work
func extractArXivID(w *workResult) string {
	// Check DOI for arXiv pattern (most reliable)
	if w.DOI != "" {
		doi := strings.TrimPrefix(w.DOI, "https://doi.org/")
		lower := strings.ToLower(doi)
		if strings.HasPrefix(lower, "10.48550/arxiv.") {
			return doi[len("10.48550/arxiv."):]
		}
	}

	// Check primary location for arXiv source
	if w.PrimaryLocation != nil && w.PrimaryLocation.Source != nil {
		srcName := strings.ToLower(w.PrimaryLocation.Source.DisplayName)
		if strings.Contains(srcName, "arxiv") && w.PrimaryLocation.LandingPageURL != "" {
			url := w.PrimaryLocation.LandingPageURL
			if idx := strings.Index(url, "/abs/"); idx != -1 {
				id := url[idx+5:]
				// Strip any trailing version like "v1"
				return strings.TrimRight(id, "/")
			}
		}
	}

	return ""
}

// extractPMID tries to extract a PubMed ID from an OpenAlex work
func extractPMID(w *workResult) string {
	if pmid, ok := w.IDs["pmid"]; ok {
		if pmidStr, ok := pmid.(string); ok {
			// Format: "https://pubmed.ncbi.nlm.nih.gov/12345678"
			id := strings.TrimPrefix(pmidStr, "https://pubmed.ncbi.nlm.nih.gov/")
			return strings.Trim(id, "/")
		}
	}
	return ""
}

// extractPMCID tries to extract a PubMed Central ID from an OpenAlex work
func extractPMCID(w *workResult) string {
	if pmcid, ok := w.IDs["pmcid"]; ok {
		if pmcidStr, ok := pmcid.(string); ok {
			// Format: "https://www.ncbi.nlm.nih.gov/pmc/articles/PMC12345/"
			id := strings.TrimPrefix(pmcidStr, "https://www.ncbi.nlm.nih.gov/pmc/articles/")
			return strings.Trim(id, "/")
		}
	}
	return ""
}

// reconstructAbstract rebuilds a plain text abstract from OpenAlex's inverted index format.
// OpenAlex stores abstracts as {"word": [position1, position2], ...}
func reconstructAbstract(invertedIndex map[string][]int) string {
	if len(invertedIndex) == 0 {
		return ""
	}

	// Find the maximum position to size the array
	maxPos := 0
	for _, positions := range invertedIndex {
		for _, pos := range positions {
			if pos > maxPos {
				maxPos = pos
			}
		}
	}

	// Build words array indexed by position
	words := make([]string, maxPos+1)
	for word, positions := range invertedIndex {
		for _, pos := range positions {
			if pos >= 0 && pos <= maxPos {
				words[pos] = word
			}
		}
	}

	// Join with spaces, filtering empty slots
	var sb strings.Builder
	for i, word := range words {
		if word != "" {
			if i > 0 && sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(word)
		}
	}

	return sb.String()
}
