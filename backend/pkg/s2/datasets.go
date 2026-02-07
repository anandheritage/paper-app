// Package s2 provides a client for the Semantic Scholar Datasets API.
// Used to download bulk academic paper data and stream it for indexing.
package s2

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const baseURL = "https://api.semanticscholar.org/datasets/v1"

// Client communicates with the Semantic Scholar Datasets API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new S2 Datasets client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for large streaming downloads
		},
	}
}

// Release represents an S2 dataset release.
type Release struct {
	ReleaseID string `json:"release_id"`
}

// Dataset contains download URLs for a specific dataset.
type Dataset struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// S2Paper represents a single paper from the S2 bulk dataset (JSONL format).
type S2Paper struct {
	CorpusID                 int                    `json:"corpusid"`
	ExternalIDs              map[string]interface{} `json:"externalids"`
	URL                      string                 `json:"url"`
	Title                    string                 `json:"title"`
	Abstract                 *string                `json:"abstract"`
	Venue                    string                 `json:"venue"`
	Year                     int                    `json:"year"`
	ReferenceCount           int                    `json:"referencecount"`
	CitationCount            int                    `json:"citationcount"`
	InfluentialCitationCount int                    `json:"influentialcitationcount"`
	IsOpenAccess             bool                   `json:"isopenaccess"`
	S2FieldsOfStudy          []S2Field              `json:"s2fieldsofstudy"`
	PublicationTypes         []string               `json:"publicationtypes"`
	PublicationDate          *string                `json:"publicationdate"`
	Journal                  *Journal               `json:"journal"`
	Authors                  []S2Author             `json:"authors"`
}

// S2Field represents a field of study classification.
type S2Field struct {
	Category string `json:"category"`
	Source   string `json:"source"`
}

// Journal represents journal publication info.
type Journal struct {
	Name   string `json:"name"`
	Volume string `json:"volume"`
	Pages  string `json:"pages"`
}

// S2Author represents a paper author.
type S2Author struct {
	AuthorID string `json:"authorId"`
	Name     string `json:"name"`
}

// GetArXivID extracts the arXiv ID from the ExternalIDs map.
func (p *S2Paper) GetArXivID() string {
	if v, ok := p.ExternalIDs["ArXiv"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// GetDOI extracts the DOI from the ExternalIDs map.
func (p *S2Paper) GetDOI() string {
	if v, ok := p.ExternalIDs["DOI"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// GetLatestRelease fetches the latest S2 dataset release info.
func (c *Client) GetLatestRelease(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/release/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get latest release: HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &release, nil
}

// GetDataset fetches download URLs for a specific dataset within a release.
func (c *Client) GetDataset(ctx context.Context, releaseID, datasetName string) (*Dataset, error) {
	url := fmt.Sprintf("%s/release/%s/dataset/%s", baseURL, releaseID, datasetName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get dataset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get dataset: HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var dataset Dataset
	if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return nil, fmt.Errorf("decode dataset: %w", err)
	}
	return &dataset, nil
}

// StreamPapersFile downloads a gzip JSONL file and streams papers through the callback.
// filterFn is called for each paper to decide whether to include it.
// callback receives matched papers in batches.
// Returns total matched papers and any error.
func (c *Client) StreamPapersFile(ctx context.Context, fileURL string, batchSize int, filterFn func(*S2Paper) bool, callback func(papers []S2Paper) error) (int, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	scanner := bufio.NewScanner(gzReader)
	scanner.Buffer(make([]byte, 0), 10*1024*1024) // 10MB max line

	batch := make([]S2Paper, 0, batchSize)
	total := 0
	scanned := 0

	for scanner.Scan() {
		scanned++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var paper S2Paper
		if err := json.Unmarshal(line, &paper); err != nil {
			continue // skip malformed lines
		}

		if filterFn != nil && !filterFn(&paper) {
			continue
		}

		batch = append(batch, paper)
		total++

		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return total, fmt.Errorf("callback error after %d papers: %w", total, err)
			}
			batch = batch[:0]

			// Progress log every 5000 matched papers
			if total%5000 == 0 {
				elapsed := time.Since(start)
				rate := float64(total) / elapsed.Seconds()
				log.Printf("  Progress: %d matched / %d scanned (%.0f matched/sec)", total, scanned, rate)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return total, fmt.Errorf("scanner error after %d papers: %w", total, err)
	}

	// Flush remaining batch
	if len(batch) > 0 {
		if err := callback(batch); err != nil {
			return total, fmt.Errorf("callback error (flush): %w", err)
		}
	}

	return total, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
