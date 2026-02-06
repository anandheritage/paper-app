// Package main provides a CLI tool to enrich locally-stored arXiv papers with
// citation counts from the OpenAlex API.
//
// It reads papers with citation_count=0 from PostgreSQL, queries OpenAlex in
// batches of 50 (using the arXiv DOI pattern), and updates the local records.
//
// Usage:
//
//	go run cmd/enrich/main.go \
//	  --db "postgres://user:pass@host:5432/paper?sslmode=disable" \
//	  --email your@email.com \
//	  --batch 50 \
//	  --limit 100000
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
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type openAlexWork struct {
	DOI          string `json:"doi"`
	CitedByCount int    `json:"cited_by_count"`
}

type openAlexResponse struct {
	Meta struct {
		Count int `json:"count"`
	} `json:"meta"`
	Results []openAlexWork `json:"results"`
}

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	email := flag.String("email", os.Getenv("OPENALEX_EMAIL"), "Email for OpenAlex polite pool (recommended)")
	batchSize := flag.Int("batch", 50, "Number of papers per OpenAlex API request")
	limitPapers := flag.Int("limit", 0, "Max papers to enrich (0 = all unenriched)")
	rateLimit := flag.Duration("rate", 100*time.Millisecond, "Delay between API requests (100ms = 10 req/sec)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://paper:paper@localhost:5432/paper?sslmode=disable"
	}

	log.Println("Connecting to database...")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Count papers needing enrichment
	var unenrichedCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM papers WHERE source = 'arxiv' AND citation_count = 0`).Scan(&unenrichedCount)
	if err != nil {
		log.Fatalf("Failed to count unenriched papers: %v", err)
	}
	log.Printf("Papers needing enrichment: %d", unenrichedCount)

	if unenrichedCount == 0 {
		log.Println("No papers need enrichment. Done!")
		return
	}

	toProcess := unenrichedCount
	if *limitPapers > 0 && *limitPapers < toProcess {
		toProcess = *limitPapers
	}
	log.Printf("Will enrich up to %d papers in batches of %d", toProcess, *batchSize)

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var (
		processed  int
		enriched   int
		notFound   int
		errors     int
		offset     int
		startTime  = time.Now()
		lastLog    = time.Now()
	)

	for processed < toProcess {
		// Fetch a batch of external_ids needing enrichment
		batchLimit := *batchSize
		if processed+batchLimit > toProcess {
			batchLimit = toProcess - processed
		}

		rows, err := pool.Query(ctx,
			`SELECT external_id FROM papers WHERE source = 'arxiv' AND citation_count = 0 ORDER BY external_id LIMIT $1 OFFSET $2`,
			batchLimit, offset,
		)
		if err != nil {
			log.Fatalf("Failed to fetch papers: %v", err)
		}

		var arxivIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				log.Printf("WARN: Scan error: %v", err)
				continue
			}
			arxivIDs = append(arxivIDs, id)
		}
		rows.Close()

		if len(arxivIDs) == 0 {
			break // no more papers
		}

		// Build DOIs for OpenAlex query
		// arXiv papers have DOIs like: 10.48550/arXiv.{id}
		var dois []string
		doiToArxiv := make(map[string]string) // lowercase DOI → arXiv ID
		for _, id := range arxivIDs {
			doi := fmt.Sprintf("10.48550/arXiv.%s", id)
			dois = append(dois, doi)
			doiToArxiv[strings.ToLower("https://doi.org/"+doi)] = id
		}

		// Query OpenAlex
		citations := queryOpenAlex(httpClient, dois, *email)

		// Update database
		for doiLower, count := range citations {
			arxivID, ok := doiToArxiv[doiLower]
			if !ok {
				continue
			}
			if count > 0 {
				_, err := pool.Exec(ctx,
					`UPDATE papers SET citation_count = $1 WHERE external_id = $2 AND source = 'arxiv'`,
					count, arxivID,
				)
				if err != nil {
					log.Printf("WARN: Failed to update %s: %v", arxivID, err)
					errors++
				} else {
					enriched++
				}
			} else {
				notFound++
			}
		}

		// Mark papers not found in OpenAlex with -1 so we don't retry them
		for _, id := range arxivIDs {
			doi := strings.ToLower("https://doi.org/10.48550/arXiv." + id)
			if _, found := citations[doi]; !found {
				// Set to -1 to mark as "checked but not found in OpenAlex"
				pool.Exec(ctx,
					`UPDATE papers SET citation_count = -1 WHERE external_id = $1 AND source = 'arxiv' AND citation_count = 0`,
					id,
				)
				notFound++
			}
		}

		processed += len(arxivIDs)
		offset += len(arxivIDs)

		// Rate limit
		time.Sleep(*rateLimit)

		// Progress log
		if time.Since(lastLog) > 10*time.Second {
			elapsed := time.Since(startTime).Seconds()
			rate := float64(processed) / elapsed
			eta := time.Duration(float64(toProcess-processed)/rate) * time.Second
			log.Printf("Progress: %d/%d processed, %d enriched, %d not found, %d errors | %.0f/sec | ETA %s",
				processed, toProcess, enriched, notFound, errors, rate, eta.Round(time.Second))
			lastLog = time.Now()
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("=== Enrichment Complete ===")
	log.Printf("Processed: %d", processed)
	log.Printf("Enriched:  %d (citation count updated)", enriched)
	log.Printf("Not found: %d (paper not in OpenAlex)", notFound)
	log.Printf("Errors:    %d", errors)
	log.Printf("Duration:  %s", elapsed.Round(time.Second))

	// Reset -1 markers back to 0 for cleanliness
	_, _ = pool.Exec(ctx, `UPDATE papers SET citation_count = 0 WHERE citation_count = -1`)

	log.Println("Done!")
}

// queryOpenAlex fetches citation counts for a batch of DOIs from OpenAlex.
// Returns a map of lowercase DOI URL → citation count.
func queryOpenAlex(client *http.Client, dois []string, email string) map[string]int {
	results := make(map[string]int)
	if len(dois) == 0 {
		return results
	}

	// Build filter: doi:10.48550/arXiv.1706.03762|10.48550/arXiv.2301.00001
	doiFilter := strings.Join(dois, "|")

	params := url.Values{}
	params.Set("filter", "doi:"+doiFilter)
	params.Set("select", "doi,cited_by_count")
	params.Set("per_page", fmt.Sprintf("%d", len(dois)))
	if email != "" {
		params.Set("mailto", email)
	}

	reqURL := fmt.Sprintf("https://api.openalex.org/works?%s", params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Printf("WARN: Failed to create request: %v", err)
		return results
	}

	ua := "PaperApp-Enricher/1.0"
	if email != "" {
		ua = fmt.Sprintf("PaperApp-Enricher/1.0 (mailto:%s)", email)
	}
	req.Header.Set("User-Agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WARN: OpenAlex request failed: %v", err)
		return results
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("WARN: OpenAlex returned %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
		return results
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("WARN: Failed to read response: %v", err)
		return results
	}

	var oaResp openAlexResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		log.Printf("WARN: Failed to parse response: %v", err)
		return results
	}

	for _, work := range oaResp.Results {
		doi := strings.ToLower(work.DOI)
		results[doi] = work.CitedByCount
	}

	return results
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
