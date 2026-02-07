// graphapi.go provides access to the Semantic Scholar Graph API.
// The bulk search endpoint works without an API key (with lower rate limits).
package s2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const graphBaseURL = "https://api.semanticscholar.org/graph/v1"

// GraphClient communicates with the Semantic Scholar Graph API.
type GraphClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewGraphClient creates a new Graph API client.
func NewGraphClient(apiKey string) *GraphClient {
	return &GraphClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GraphPaper represents a paper from the S2 Graph API with all requested fields.
type GraphPaper struct {
	PaperID        string                 `json:"paperId"`
	CorpusID       int                    `json:"corpusId"`
	ExternalIDs    map[string]interface{} `json:"externalIds"`
	URL            string                 `json:"url"`
	Title          string                 `json:"title"`
	Abstract       *string                `json:"abstract"`
	Venue          string                 `json:"venue"`
	Year           int                    `json:"year"`
	ReferenceCount int                    `json:"referenceCount"`
	CitationCount  int                    `json:"citationCount"`
	InfluentialCitationCount int          `json:"influentialCitationCount"`
	IsOpenAccess   bool                   `json:"isOpenAccess"`
	OpenAccessPdf  *struct {
		URL    string `json:"url"`
		Status string `json:"status"`
	} `json:"openAccessPdf"`
	S2FieldsOfStudy []S2Field            `json:"s2FieldsOfStudy"`
	PublicationTypes []string             `json:"publicationTypes"`
	PublicationDate *string               `json:"publicationDate"`
	Journal        *Journal               `json:"journal"`
	Authors        []S2Author             `json:"authors"`
	TLDR           *struct {
		Model string `json:"model"`
		Text  string `json:"text"`
	} `json:"tldr"`
}

// GetArXivID extracts the arXiv ID from the ExternalIDs map.
func (p *GraphPaper) GetArXivID() string {
	if v, ok := p.ExternalIDs["ArXiv"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// GetDOI extracts the DOI from the ExternalIDs map.
func (p *GraphPaper) GetDOI() string {
	if v, ok := p.ExternalIDs["DOI"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// BulkSearchResult represents a response from /paper/search/bulk.
type BulkSearchResult struct {
	Total int          `json:"total"`
	Token string       `json:"token"` // Empty when no more results
	Data  []GraphPaper `json:"data"`
}

// allFields is the list of fields we request from the Graph API.
const allFields = "title,abstract,venue,year,referenceCount,citationCount,influentialCitationCount,isOpenAccess,openAccessPdf,s2FieldsOfStudy,publicationTypes,publicationDate,journal,authors,externalIds,url,corpusId,tldr"

// BulkSearch performs a single bulk search request.
// Returns the result including a continuation token for pagination.
func (c *GraphClient) BulkSearch(ctx context.Context, query string, token string) (*BulkSearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("fields", allFields)
	params.Set("limit", "1000")
	if token != "" {
		params.Set("token", token)
	}

	reqURL := fmt.Sprintf("%s/paper/search/bulk?%s", graphBaseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bulk search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (429)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bulk search failed (HTTP %d): %s", resp.StatusCode, truncateStr(string(body), 300))
	}

	var result BulkSearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// BatchPaper fetches papers by IDs using /paper/batch.
// ids can be S2 paper IDs, arXiv IDs (prefix "ArXiv:"), DOIs (prefix "DOI:"), etc.
// Max 500 IDs per request.
func (c *GraphClient) BatchPaper(ctx context.Context, ids []string) ([]GraphPaper, error) {
	if len(ids) > 500 {
		return nil, fmt.Errorf("max 500 IDs per batch, got %d", len(ids))
	}

	reqURL := fmt.Sprintf("%s/paper/batch?fields=%s", graphBaseURL, allFields)

	payload := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (429)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch fetch failed (HTTP %d): %s", resp.StatusCode, truncateStr(string(body), 300))
	}

	var papers []GraphPaper
	if err := json.Unmarshal(body, &papers); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return papers, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
