// Package opensearch provides a lightweight client for AWS OpenSearch Service.
// Uses raw HTTP (no external dependencies) for full control over queries and mappings.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Config holds OpenSearch connection settings.
type Config struct {
	Endpoint string // e.g. "https://search-xxx.us-east-1.es.amazonaws.com"
	Index    string // e.g. "papers"
	Username string // for fine-grained access control (optional)
	Password string // for fine-grained access control (optional)
}

// Client communicates with an OpenSearch cluster.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient creates a new OpenSearch client.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ---------- Index Management ----------

// IndexMapping defines the OpenSearch index mapping for papers.
// Optimized for search relevance, category filtering, and sorting.
const IndexMapping = `{
  "settings": {
    "number_of_shards": 2,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "paper_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "stop", "snowball"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id":               { "type": "keyword" },
      "external_id":      { "type": "keyword" },
      "source":           { "type": "keyword" },
      "title":            { "type": "text", "analyzer": "paper_analyzer", "fields": { "keyword": { "type": "keyword", "ignore_above": 512 } } },
      "abstract":         { "type": "text", "analyzer": "paper_analyzer" },
      "authors": {
        "type": "nested",
        "properties": {
          "name":        { "type": "text", "fields": { "keyword": { "type": "keyword" } } },
          "affiliation": { "type": "text" }
        }
      },
      "primary_category": { "type": "keyword" },
      "categories":       { "type": "keyword" },
      "published_date":   { "type": "date", "format": "yyyy-MM-dd||epoch_millis" },
      "updated_date":     { "type": "date", "format": "yyyy-MM-dd||epoch_millis" },
      "citation_count":   { "type": "integer" },
      "doi":              { "type": "keyword" },
      "journal_ref":      { "type": "text" },
      "comments":         { "type": "text" },
      "license":          { "type": "keyword" },
      "pdf_url":          { "type": "keyword", "index": false }
    }
  }
}`

// CreateIndex creates the papers index with the proper mapping.
func (c *Client) CreateIndex(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s", c.cfg.Endpoint, c.cfg.Index)
	resp, err := c.doRequest(ctx, "PUT", url, []byte(IndexMapping))
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Printf("[OpenSearch] Index '%s' created", c.cfg.Index)
		return nil
	}

	// 400 = index already exists (resource_already_exists_exception)
	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "resource_already_exists_exception") {
			log.Printf("[OpenSearch] Index '%s' already exists", c.cfg.Index)
			return nil
		}
		return fmt.Errorf("create index failed (400): %s", string(body[:min(500, len(body))]))
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create index failed (%d): %s", resp.StatusCode, string(body[:min(500, len(body))]))
}

// DeleteIndex deletes the papers index.
func (c *Client) DeleteIndex(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s", c.cfg.Endpoint, c.cfg.Index)
	resp, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete index failed (%d): %s", resp.StatusCode, string(body[:min(500, len(body))]))
}

// ---------- Document Operations ----------

// PaperDoc is the document structure stored in OpenSearch.
type PaperDoc struct {
	ID              string      `json:"id"`
	ExternalID      string      `json:"external_id"`
	Source          string      `json:"source"`
	Title           string      `json:"title"`
	Abstract        string      `json:"abstract"`
	Authors         interface{} `json:"authors"`
	PrimaryCategory string      `json:"primary_category"`
	Categories      []string    `json:"categories"`
	PublishedDate   *string     `json:"published_date,omitempty"`
	UpdatedDate     *string     `json:"updated_date,omitempty"`
	CitationCount   int         `json:"citation_count"`
	DOI             string      `json:"doi,omitempty"`
	JournalRef      string      `json:"journal_ref,omitempty"`
	Comments        string      `json:"comments,omitempty"`
	License         string      `json:"license,omitempty"`
	PDFURL          string      `json:"pdf_url,omitempty"`
}

// IndexDoc indexes a single document.
func (c *Client) IndexDoc(ctx context.Context, doc *PaperDoc) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal doc: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_doc/%s", c.cfg.Endpoint, c.cfg.Index, doc.ID)
	resp, err := c.doRequest(ctx, "PUT", url, body)
	if err != nil {
		return fmt.Errorf("index doc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index doc failed (%d): %s", resp.StatusCode, string(respBody[:min(300, len(respBody))]))
	}

	return nil
}

// BulkIndex indexes multiple documents using the _bulk API.
// Returns the number of successfully indexed documents.
func (c *Client) BulkIndex(ctx context.Context, docs []*PaperDoc) (int, error) {
	if len(docs) == 0 {
		return 0, nil
	}

	var buf bytes.Buffer
	for _, doc := range docs {
		// Action line
		action := map[string]interface{}{
			"index": map[string]string{
				"_index": c.cfg.Index,
				"_id":    doc.ID,
			},
		}
		actionJSON, _ := json.Marshal(action)
		buf.Write(actionJSON)
		buf.WriteByte('\n')

		// Document line
		docJSON, err := json.Marshal(doc)
		if err != nil {
			continue
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/_bulk", c.cfg.Endpoint)
	resp, err := c.doRequest(ctx, "POST", url, buf.Bytes())
	if err != nil {
		return 0, fmt.Errorf("bulk index: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read bulk response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bulk index failed (%d): %s", resp.StatusCode, string(respBody[:min(500, len(respBody))]))
	}

	// Parse bulk response to count successes
	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int `json:"status"`
			} `json:"index"`
		} `json:"items"`
	}
	if err := json.Unmarshal(respBody, &bulkResp); err != nil {
		return len(docs), nil // Assume all succeeded if we can't parse
	}

	success := 0
	for _, item := range bulkResp.Items {
		if item.Index.Status == 200 || item.Index.Status == 201 {
			success++
		}
	}

	return success, nil
}

// ---------- Search ----------

// SearchParams defines search parameters.
type SearchParams struct {
	Query      string
	Categories []string
	SortBy     string // "relevance", "citations", "date"
	Limit      int
	Offset     int
}

// SearchResult is the result of a search operation.
type SearchResult struct {
	Hits  []*SearchHit `json:"hits"`
	Total int          `json:"total"`
}

// SearchHit is a single search result.
type SearchHit struct {
	Doc   PaperDoc `json:"doc"`
	Score float64  `json:"score"`
}

// Search performs a full-text search with optional category filtering and sorting.
func (c *Client) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	query := c.buildSearchQuery(params)

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.cfg.Endpoint, c.cfg.Index)
	resp, err := c.doRequest(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed (%d): %s", resp.StatusCode, string(respBody[:min(500, len(respBody))]))
	}

	var esResp struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source PaperDoc `json:"_source"`
				Score  float64 `json:"_score"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(respBody, &esResp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	result := &SearchResult{
		Total: esResp.Hits.Total.Value,
	}
	for _, hit := range esResp.Hits.Hits {
		result.Hits = append(result.Hits, &SearchHit{
			Doc:   hit.Source,
			Score: hit.Score,
		})
	}

	return result, nil
}

// buildSearchQuery constructs the OpenSearch query DSL.
func (c *Client) buildSearchQuery(params SearchParams) map[string]interface{} {
	query := map[string]interface{}{
		"from": params.Offset,
		"size": params.Limit,
	}

	// Build the query part
	var must []interface{}
	var filter []interface{}

	if params.Query != "" {
		// Multi-match across title (boosted), abstract, authors
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  params.Query,
				"fields": []string{"title^3", "title.keyword^5", "abstract", "authors.name^2"},
				"type":   "best_fields",
				"fuzziness": "AUTO",
			},
		})
	}

	if len(params.Categories) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{
				"categories": params.Categories,
			},
		})
	}

	boolQuery := map[string]interface{}{}
	if len(must) > 0 {
		boolQuery["must"] = must
	}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}

	if len(boolQuery) > 0 {
		query["query"] = map[string]interface{}{
			"bool": boolQuery,
		}
	} else {
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	// Sorting
	switch params.SortBy {
	case "citations":
		query["sort"] = []interface{}{
			map[string]interface{}{"citation_count": map[string]string{"order": "desc"}},
			"_score",
			map[string]interface{}{"published_date": map[string]string{"order": "desc", "missing": "_last"}},
		}
	case "date":
		query["sort"] = []interface{}{
			map[string]interface{}{"published_date": map[string]string{"order": "desc", "missing": "_last"}},
			"_score",
		}
	default: // relevance
		if params.Query != "" {
			// Default scoring + citation boost
			query["sort"] = []interface{}{
				"_score",
				map[string]interface{}{"citation_count": map[string]string{"order": "desc"}},
			}
		} else {
			query["sort"] = []interface{}{
				map[string]interface{}{"published_date": map[string]string{"order": "desc", "missing": "_last"}},
			}
		}
	}

	// Highlight
	if params.Query != "" {
		query["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"title":    map[string]interface{}{"number_of_fragments": 0},
				"abstract": map[string]interface{}{"fragment_size": 200, "number_of_fragments": 1},
			},
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
		}
	}

	return query
}

// GetCategoryCounts returns aggregated paper counts per category.
func (c *Client) GetCategoryCounts(ctx context.Context) (map[string]int64, error) {
	query := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"categories": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "primary_category",
					"size":  200,
				},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_search", c.cfg.Endpoint, c.cfg.Index)
	resp, err := c.doRequest(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("category counts: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("category counts failed (%d): %s", resp.StatusCode, string(respBody[:min(300, len(respBody))]))
	}

	var esResp struct {
		Aggregations struct {
			Categories struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"categories"`
		} `json:"aggregations"`
	}
	if err := json.Unmarshal(respBody, &esResp); err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, b := range esResp.Aggregations.Categories.Buckets {
		counts[b.Key] = b.DocCount
	}
	return counts, nil
}

// Ping checks if the OpenSearch cluster is reachable.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", c.cfg.Endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---------- HTTP helper ----------

func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.Username != "" && c.cfg.Password != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	return c.httpClient.Do(req)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
