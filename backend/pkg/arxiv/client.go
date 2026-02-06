package arxiv

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paper-app/backend/internal/domain"
)

const baseURL = "http://export.arxiv.org/api/query"

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type SearchResult struct {
	Papers       []*domain.Paper
	TotalResults int
}

// Feed represents the arXiv Atom feed response
type Feed struct {
	XMLName      xml.Name `xml:"feed"`
	TotalResults int      `xml:"totalResults"`
	Entries      []Entry  `xml:"entry"`
}

type Entry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Summary   string     `xml:"summary"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Authors   []Author   `xml:"author"`
	Links     []Link     `xml:"link"`
	Category  []Category `xml:"category"`
}

type Author struct {
	Name        string `xml:"name"`
	Affiliation string `xml:"affiliation"`
}

type Link struct {
	Href  string `xml:"href,attr"`
	Rel   string `xml:"rel,attr"`
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
}

type Category struct {
	Term string `xml:"term,attr"`
}

func (c *Client) Search(query string, limit, offset int) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("search_query", fmt.Sprintf("all:%s", query))
	params.Set("start", fmt.Sprintf("%d", offset))
	params.Set("max_results", fmt.Sprintf("%d", limit))
	params.Set("sortBy", "relevance")
	params.Set("sortOrder", "descending")

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("arxiv API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read arxiv response: %w", err)
	}

	var feed Feed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse arxiv response: %w", err)
	}

	papers := make([]*domain.Paper, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		paper := entryToPaper(&entry)
		if paper != nil {
			papers = append(papers, paper)
		}
	}

	return &SearchResult{
		Papers:       papers,
		TotalResults: feed.TotalResults,
	}, nil
}

func (c *Client) GetPaper(arxivID string) (*domain.Paper, error) {
	params := url.Values{}
	params.Set("id_list", arxivID)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("arxiv API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read arxiv response: %w", err)
	}

	var feed Feed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse arxiv response: %w", err)
	}

	if len(feed.Entries) == 0 {
		return nil, nil
	}

	return entryToPaper(&feed.Entries[0]), nil
}

func entryToPaper(entry *Entry) *domain.Paper {
	// Extract arXiv ID from the full URL
	// e.g., "http://arxiv.org/abs/2301.00001v1" -> "2301.00001"
	arxivID := extractArxivID(entry.ID)
	if arxivID == "" {
		return nil
	}

	// Parse authors
	authors := make([]domain.Author, 0, len(entry.Authors))
	for _, a := range entry.Authors {
		authors = append(authors, domain.Author{
			Name:        strings.TrimSpace(a.Name),
			Affiliation: strings.TrimSpace(a.Affiliation),
		})
	}

	authorsJSON, _ := json.Marshal(authors)

	// Parse published date
	var publishedDate *time.Time
	if entry.Published != "" {
		if t, err := time.Parse(time.RFC3339, entry.Published); err == nil {
			publishedDate = &t
		}
	}

	// Extract PDF URL
	pdfURL := fmt.Sprintf("https://arxiv.org/pdf/%s", arxivID)
	for _, link := range entry.Links {
		if link.Title == "pdf" || link.Type == "application/pdf" {
			pdfURL = link.Href
			break
		}
	}

	// Extract categories as metadata
	categories := make([]string, 0, len(entry.Category))
	for _, cat := range entry.Category {
		categories = append(categories, cat.Term)
	}
	metadata := map[string]interface{}{
		"categories": categories,
		"html_url":   fmt.Sprintf("https://ar5iv.labs.arxiv.org/html/%s", arxivID),
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &domain.Paper{
		ExternalID:    arxivID,
		Source:        "arxiv",
		Title:         strings.TrimSpace(entry.Title),
		Abstract:      strings.TrimSpace(entry.Summary),
		Authors:       authorsJSON,
		PublishedDate: publishedDate,
		PDFURL:        pdfURL,
		Metadata:      metadataJSON,
	}
}

func extractArxivID(fullURL string) string {
	// Handle formats like:
	// "http://arxiv.org/abs/2301.00001v1"
	// "http://arxiv.org/abs/hep-th/9901001v1"
	parts := strings.Split(fullURL, "/abs/")
	if len(parts) != 2 {
		return ""
	}
	id := parts[1]
	// Remove version suffix
	if idx := strings.LastIndex(id, "v"); idx > 0 {
		// Check if everything after 'v' is a number
		versionPart := id[idx+1:]
		isVersion := true
		for _, c := range versionPart {
			if c < '0' || c > '9' {
				isVersion = false
				break
			}
		}
		if isVersion && len(versionPart) > 0 {
			id = id[:idx]
		}
	}
	return id
}
