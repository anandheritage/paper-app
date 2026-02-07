// oaimport downloads arXiv paper metadata from OpenAlex (free, no key needed)
// and indexes it directly into OpenSearch with citation counts, abstracts, authors, etc.
//
// OpenAlex has 3.3M+ arXiv papers with citation data. This tool:
//   1. Queries the OpenAlex API with cursor pagination
//   2. Extracts arXiv papers with full metadata
//   3. Bulk-indexes into OpenSearch
//
// Usage:
//   oaimport --opensearch=http://localhost:9200 --recreate-index
//   oaimport --opensearch=http://localhost:9200 --mailto=you@email.com
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/paper-app/backend/pkg/opensearch"
)

// ---------- OpenAlex API types ----------

type oaResponse struct {
	Meta    oaMeta   `json:"meta"`
	Results []oaWork `json:"results"`
}

type oaMeta struct {
	Count      int     `json:"count"`
	PerPage    int     `json:"per_page"`
	NextCursor *string `json:"next_cursor"`
}

type oaWork struct {
	ID                    string                     `json:"id"`
	Title                 string                     `json:"title"`
	AbstractInvertedIndex map[string][]int           `json:"abstract_inverted_index"`
	CitedByCount          int                        `json:"cited_by_count"`
	PublicationDate       string                     `json:"publication_date"`
	PublicationYear       int                        `json:"publication_year"`
	DOI                   string                     `json:"doi"`
	Type                  string                     `json:"type"`
	Locations             []oaLocation               `json:"locations"`
	Authorships           []oaAuthorship             `json:"authorships"`
	Topics                []oaTopic                  `json:"topics"`
	OpenAccess            oaOpenAccess               `json:"open_access"`
	IDs                   map[string]interface{}      `json:"ids"`
}

type oaLocation struct {
	LandingPageURL string    `json:"landing_page_url"`
	PDFURL         *string   `json:"pdf_url"`
	Source         *oaSource `json:"source"`
}

type oaSource struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type oaAuthorship struct {
	Author       oaAuthor        `json:"author"`
	Institutions []oaInstitution `json:"institutions"`
}

type oaAuthor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type oaInstitution struct {
	DisplayName string `json:"display_name"`
}

type oaTopic struct {
	DisplayName string  `json:"display_name"`
	Score       float64 `json:"score"`
	Subfield    *struct {
		DisplayName string `json:"display_name"`
	} `json:"subfield"`
	Field *struct {
		DisplayName string `json:"display_name"`
	} `json:"field"`
	Domain *struct {
		DisplayName string `json:"display_name"`
	} `json:"domain"`
}

type oaOpenAccess struct {
	IsOA     bool   `json:"is_oa"`
	OAStatus string `json:"oa_status"`
}

// ---------- Main ----------

// Multiple patterns to extract arXiv IDs from different URL formats:
//   arxiv.org/abs/2301.01234   – canonical
//   arxiv.org/pdf/2301.01234   – PDF link used as landing page
//   export.arxiv.org/pdf/2301.01234 – export mirror
//   doi.org/10.48550/arxiv.2301.01234 – DOI-based
var (
	arxivAbsRegex = regexp.MustCompile(`arxiv\.org/abs/([0-9]+\.[0-9]+)`)
	arxivPDFRegex = regexp.MustCompile(`arxiv\.org/pdf/([0-9]+\.[0-9]+)`)
	arxivDOIRegex = regexp.MustCompile(`10\.48550/arxiv\.([0-9]+\.[0-9]+)`)
	// Older arXiv IDs like hep-ph/9901234
	arxivOldAbsRegex = regexp.MustCompile(`arxiv\.org/abs/([a-z-]+/[0-9]+)`)
	arxivOldPDFRegex = regexp.MustCompile(`arxiv\.org/pdf/([a-z-]+/[0-9]+)`)
)

func main() {
	osEndpoint := flag.String("opensearch", envOrDefault("OPENSEARCH_ENDPOINT", "http://localhost:9200"), "OpenSearch endpoint URL")
	osIndex := flag.String("index", "papers", "OpenSearch index name")
	recreate := flag.Bool("recreate-index", false, "Delete and recreate index before import")
	batchSize := flag.Int("batch-size", 500, "Bulk index batch size")
	perPage := flag.Int("per-page", 200, "Results per API page (max 200)")
	mailto := flag.String("mailto", envOrDefault("OPENALEX_MAILTO", "admin@dapapers.com"), "Email for OpenAlex polite pool (faster rate limits)")
	startCursor := flag.String("cursor", "*", "Starting cursor (use * for beginning, or paste a cursor to resume)")
	flag.Parse()

	if *osEndpoint == "" {
		log.Fatal("OPENSEARCH_ENDPOINT is required")
	}

	log.Printf("OpenSearch: %s/%s", *osEndpoint, *osIndex)
	log.Printf("OpenAlex mailto: %s (polite pool = ~10 req/sec)", *mailto)

	osClient := opensearch.NewClient(opensearch.Config{
		Endpoint: strings.TrimRight(*osEndpoint, "/"),
		Index:    *osIndex,
	})

	ctx := context.Background()

	// Setup index
	if *recreate {
		log.Println("Deleting existing index...")
		if err := osClient.DeleteIndex(ctx); err != nil {
			log.Printf("WARNING: Delete index failed: %v", err)
		}
		time.Sleep(time.Second)
	}
	log.Println("Creating index (if needed)...")
	if err := osClient.CreateIndex(ctx); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Current index doc count: %d", count)
	}

	// Build base URL
	baseURL := "https://api.openalex.org/works"
	params := url.Values{}
	params.Set("filter", "locations.source.id:S4306400194") // arXiv source ID in OpenAlex
	params.Set("per_page", strconv.Itoa(*perPage))
	params.Set("sort", "cited_by_count:desc") // Most cited first → site is useful within minutes
	params.Set("select", "id,title,abstract_inverted_index,authorships,cited_by_count,publication_date,publication_year,doi,locations,topics,type,open_access")
	if *mailto != "" {
		params.Set("mailto", *mailto)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}

	cursor := *startCursor
	totalIndexed := 0
	totalErrors := 0
	totalPages := 0
	importStart := time.Now()
	var totalPapers int

	log.Println("Starting OpenAlex import of arXiv papers...")

	for {
		params.Set("cursor", cursor)
		reqURL := baseURL + "?" + params.Encode()

		// Fetch page
		var resp oaResponse
		var fetchErr error
		for retries := 0; retries < 5; retries++ {
			resp, fetchErr = fetchPage(httpClient, reqURL)
			if fetchErr == nil {
				break
			}
			if strings.Contains(fetchErr.Error(), "429") {
				wait := time.Duration(5*(retries+1)) * time.Second
				log.Printf("Rate limited, waiting %v...", wait)
				time.Sleep(wait)
				continue
			}
			log.Printf("Fetch error (retry %d/5): %v", retries+1, fetchErr)
			time.Sleep(2 * time.Second)
		}
		if fetchErr != nil {
			log.Printf("FATAL: Failed after 5 retries: %v", fetchErr)
			log.Printf("Resume with --cursor=%s", cursor)
			break
		}

		if totalPages == 0 {
			totalPapers = resp.Meta.Count
			log.Printf("Total arXiv papers in OpenAlex: %d", totalPapers)
			log.Printf("Estimated pages: %d", (totalPapers+*perPage-1)/ *perPage)
		}

		// DEBUG: Log first paper on each page to verify sort order
		if len(resp.Results) > 0 && (totalPages < 3 || totalPages%100 == 0) {
			first := resp.Results[0]
			log.Printf("DEBUG Page %d first paper: cit=%d title=%q id=%s",
				totalPages+1, first.CitedByCount, truncate(first.Title, 50), first.ID)
		}

		// Convert to PaperDocs
		var docs []*opensearch.PaperDoc
		skipped := 0
		for i := range resp.Results {
			doc := convertOAWork(&resp.Results[i])
			if doc != nil {
				docs = append(docs, doc)
			} else {
				skipped++
				if totalPages < 3 {
					log.Printf("DEBUG SKIPPED: title=%q cit=%d id=%s",
						truncate(resp.Results[i].Title, 50), resp.Results[i].CitedByCount, resp.Results[i].ID)
				}
			}
		}
		if totalPages < 3 {
			log.Printf("DEBUG Page %d: %d results, %d converted, %d skipped",
				totalPages+1, len(resp.Results), len(docs), skipped)
		}

		// Bulk index in sub-batches
		for start := 0; start < len(docs); start += *batchSize {
			end := start + *batchSize
			if end > len(docs) {
				end = len(docs)
			}
			indexed, err := osClient.BulkIndex(ctx, docs[start:end])
			if err != nil {
				log.Printf("ERROR bulk indexing: %v", err)
				totalErrors += end - start
			} else {
				totalIndexed += indexed
				if indexed < end-start {
					totalErrors += (end - start) - indexed
				}
			}
		}

		totalPages++

		// Progress logging
		if totalPages%50 == 0 || totalPages <= 5 {
			elapsed := time.Since(importStart)
			pct := float64(totalIndexed) / float64(totalPapers) * 100
			rate := float64(totalIndexed) / elapsed.Seconds()
			eta := time.Duration(float64(totalPapers-totalIndexed)/rate) * time.Second
			log.Printf("Page %d | Indexed: %d/%d (%.1f%%) | Errors: %d | Rate: %.0f/sec | ETA: %v | Cursor: %s",
				totalPages, totalIndexed, totalPapers, pct, totalErrors, rate, eta.Round(time.Second),
				truncate(cursor, 20))
		}

		// Next page
		if resp.Meta.NextCursor == nil || *resp.Meta.NextCursor == "" || len(resp.Results) == 0 {
			log.Println("No more results — import complete!")
			break
		}
		cursor = *resp.Meta.NextCursor

		// Polite rate limiting: ~10 req/sec with mailto, ~1 req/sec without
		time.Sleep(120 * time.Millisecond)
	}

	totalElapsed := time.Since(importStart)
	log.Printf("\n========================================")
	log.Printf("OpenAlex Import Complete!")
	log.Printf("Total papers indexed: %d", totalIndexed)
	log.Printf("Total errors: %d", totalErrors)
	log.Printf("Total time: %v", totalElapsed.Round(time.Second))
	if totalElapsed.Seconds() > 0 {
		log.Printf("Rate: %.0f papers/sec", float64(totalIndexed)/totalElapsed.Seconds())
	}
	log.Printf("========================================")

	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Final index doc count: %d", count)
	}
}

// ---------- API fetch ----------

func fetchPage(client *http.Client, url string) (oaResponse, error) {
	var result oaResponse

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "DAPapers/1.0 (mailto:admin@dapapers.com)")

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return result, fmt.Errorf("429 rate limited")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return result, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("read body: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("decode: %w", err)
	}

	return result, nil
}

// ---------- Conversion ----------

func convertOAWork(w *oaWork) *opensearch.PaperDoc {
	if w.Title == "" {
		return nil
	}

	// Extract arXiv ID and PDF URL from locations
	arxivID, pdfURL := extractArxivInfo(w)
	if arxivID == "" {
		return nil // Skip non-arXiv papers (shouldn't happen with our filter, but safety check)
	}

	// Use OpenAlex work ID (numeric part) as the document ID for deduplication
	oaID := w.ID
	if strings.HasPrefix(oaID, "https://openalex.org/W") {
		oaID = strings.TrimPrefix(oaID, "https://openalex.org/W")
	}

	// Reconstruct abstract from inverted index
	abstract := reconstructAbstract(w.AbstractInvertedIndex)

	// Authors
	authors := make([]map[string]string, 0, len(w.Authorships))
	for _, a := range w.Authorships {
		author := map[string]string{"name": a.Author.DisplayName}
		if len(a.Institutions) > 0 {
			author["affiliation"] = a.Institutions[0].DisplayName
		}
		authors = append(authors, author)
	}

	// Categories from topics
	var categories []string
	seen := map[string]bool{}
	for _, t := range w.Topics {
		if t.Field != nil && !seen[t.Field.DisplayName] {
			categories = append(categories, t.Field.DisplayName)
			seen[t.Field.DisplayName] = true
		}
	}
	var primaryCategory string
	if len(categories) > 0 {
		primaryCategory = categories[0]
	}

	// Published date
	var pubDate *string
	if w.PublicationDate != "" {
		pubDate = &w.PublicationDate
	}

	// DOI — strip URL prefix
	doi := w.DOI
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://doi.org/")

	// Venue from primary location source
	venue := ""
	for _, loc := range w.Locations {
		if loc.Source != nil && loc.Source.DisplayName != "" && !strings.Contains(strings.ToLower(loc.Source.DisplayName), "arxiv") {
			venue = loc.Source.DisplayName
			break
		}
	}

	return &opensearch.PaperDoc{
		ID:              oaID,
		ExternalID:      arxivID,
		Source:          "arxiv",
		Title:           w.Title,
		Abstract:        abstract,
		Authors:         authors,
		PublishedDate:   pubDate,
		Year:            w.PublicationYear,
		PDFURL:          pdfURL,
		PrimaryCategory: primaryCategory,
		Categories:      categories,
		DOI:             doi,
		CitationCount:   w.CitedByCount,
		Venue:           venue,
		S2URL:           "", // No S2 URL from OpenAlex
		IsOpenAccess:    w.OpenAccess.IsOA,
	}
}

func extractArxivInfo(w *oaWork) (arxivID string, pdfURL string) {
	// Pass 1: Try to extract arXiv ID from all location URLs
	for _, loc := range w.Locations {
		lpu := strings.ToLower(loc.LandingPageURL)
		if lpu == "" {
			continue
		}

		// Try each pattern in order of reliability
		for _, re := range []*regexp.Regexp{arxivAbsRegex, arxivPDFRegex, arxivOldAbsRegex, arxivOldPDFRegex} {
			if m := re.FindStringSubmatch(lpu); len(m) > 1 {
				arxivID = m[1]
				if loc.PDFURL != nil && *loc.PDFURL != "" {
					pdfURL = *loc.PDFURL
				}
				break
			}
		}
		if arxivID != "" {
			break
		}
	}

	// Pass 2: Try DOI-based arXiv extraction (e.g. doi.org/10.48550/arxiv.2301.01234)
	if arxivID == "" {
		doi := strings.ToLower(w.DOI)
		if m := arxivDOIRegex.FindStringSubmatch(doi); len(m) > 1 {
			arxivID = m[1]
		}
		// Also check locations for DOI links
		if arxivID == "" {
			for _, loc := range w.Locations {
				lpu := strings.ToLower(loc.LandingPageURL)
				if m := arxivDOIRegex.FindStringSubmatch(lpu); len(m) > 1 {
					arxivID = m[1]
					break
				}
			}
		}
	}

	// Construct PDF URL if we have an ID but no PDF
	if arxivID != "" && pdfURL == "" {
		pdfURL = "https://arxiv.org/pdf/" + arxivID
	}

	return
}

func reconstructAbstract(invertedIndex map[string][]int) string {
	if len(invertedIndex) == 0 {
		return ""
	}

	type wordPos struct {
		pos  int
		word string
	}

	var pairs []wordPos
	for word, positions := range invertedIndex {
		for _, pos := range positions {
			pairs = append(pairs, wordPos{pos: pos, word: word})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].pos < pairs[j].pos
	})

	var words []string
	for _, p := range pairs {
		words = append(words, p.word)
	}

	return strings.Join(words, " ")
}

// ---------- Helpers ----------

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
