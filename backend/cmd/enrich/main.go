// Package main provides a CLI tool to enrich locally-stored arXiv papers with
// citation counts from the Semantic Scholar API.
//
// It reads papers with citation_count=0 from PostgreSQL, queries Semantic
// Scholar in batches of 500 (using arXiv IDs), and updates the local records.
//
// This is a ONE-TIME batch job — it does not run during search or page loads.
//
// Usage:
//
//	go run cmd/enrich/main.go \
//	  --db "postgres://user:pass@host:5432/paper?sslmode=disable" \
//	  --batch 500 \
//	  --limit 100000
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Semantic Scholar batch response item
type s2Paper struct {
	PaperID       string `json:"paperId"`
	ExternalIDs   *struct {
		ArXiv string `json:"ArXiv"`
	} `json:"externalIds"`
	CitationCount int `json:"citationCount"`
}

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	batchSize := flag.Int("batch", 500, "Number of papers per Semantic Scholar batch (max 500)")
	limitPapers := flag.Int("limit", 0, "Max papers to enrich (0 = all unenriched)")
	rateDelay := flag.Duration("rate", 1050*time.Millisecond, "Delay between API requests (Semantic Scholar: 1 req/sec unauthenticated)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://paper:paper@localhost:5432/paper?sslmode=disable"
	}
	if *batchSize > 500 {
		*batchSize = 500 // Semantic Scholar max
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
	log.Printf("Papers needing citation enrichment: %d", unenrichedCount)

	if unenrichedCount == 0 {
		log.Println("All papers already enriched. Done!")
		return
	}

	toProcess := unenrichedCount
	if *limitPapers > 0 && *limitPapers < toProcess {
		toProcess = *limitPapers
	}

	estimateRequests := (toProcess + *batchSize - 1) / *batchSize
	estimateDuration := time.Duration(estimateRequests) * *rateDelay
	log.Printf("Will enrich up to %d papers in %d batches (~%s)", toProcess, estimateRequests, estimateDuration.Round(time.Second))

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var (
		processed int
		enriched  int
		notFound  int
		apiErrors int
		startTime = time.Now()
		lastLog   = time.Now()
	)

	for processed < toProcess {
		batchLimit := *batchSize
		if processed+batchLimit > toProcess {
			batchLimit = toProcess - processed
		}

		// Fetch a batch of arXiv IDs needing enrichment
		rows, err := pool.Query(ctx,
			`SELECT external_id FROM papers WHERE source = 'arxiv' AND citation_count = 0 ORDER BY external_id LIMIT $1`,
			batchLimit,
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
			break
		}

		// Query Semantic Scholar batch API
		citations, err := querySemantic(httpClient, arxivIDs)
		if err != nil {
			log.Printf("WARN: Semantic Scholar batch failed: %v", err)
			apiErrors++
			// Wait longer on error (rate limit backoff)
			time.Sleep(5 * time.Second)
			continue
		}

		// Update database in a single batch
		batch := &pgx.Batch{}
		for _, id := range arxivIDs {
			count, found := citations[id]
			if found && count > 0 {
				batch.Queue(`UPDATE papers SET citation_count = $1 WHERE external_id = $2 AND source = 'arxiv'`, count, id)
				enriched++
			} else {
				// Mark as checked (-1) so we skip it next time
				batch.Queue(`UPDATE papers SET citation_count = -1 WHERE external_id = $1 AND source = 'arxiv' AND citation_count = 0`, id)
				notFound++
			}
		}

		batchCtx, batchCancel := context.WithTimeout(ctx, 60*time.Second)
		results := pool.SendBatch(batchCtx, batch)
		results.Close()
		batchCancel()

		processed += len(arxivIDs)

		// Rate limit
		time.Sleep(*rateDelay)

		// Progress log every 10 seconds
		if time.Since(lastLog) > 10*time.Second {
			elapsed := time.Since(startTime).Seconds()
			rate := float64(processed) / elapsed
			remaining := toProcess - processed
			eta := time.Duration(float64(remaining)/rate) * time.Second
			log.Printf("Progress: %d/%d (%.1f%%) | enriched: %d | not found: %d | errors: %d | %.0f papers/sec | ETA: %s",
				processed, toProcess, float64(processed)/float64(toProcess)*100,
				enriched, notFound, apiErrors, rate, eta.Round(time.Second))
			lastLog = time.Now()
		}
	}

	elapsed := time.Since(startTime)
	log.Println("=== Enrichment Complete ===")
	log.Printf("Processed:  %d", processed)
	log.Printf("Enriched:   %d (got citation counts)", enriched)
	log.Printf("Not found:  %d (not in Semantic Scholar)", notFound)
	log.Printf("API errors: %d", apiErrors)
	log.Printf("Duration:   %s", elapsed.Round(time.Second))

	// Reset -1 markers back to 0 for display
	log.Println("Resetting temporary markers...")
	_, _ = pool.Exec(ctx, `UPDATE papers SET citation_count = 0 WHERE citation_count = -1`)

	log.Println("Done! Citation sorting is now available.")
}

// querySemantic queries Semantic Scholar's batch paper endpoint.
// Input: slice of arXiv IDs (e.g. "1706.03762")
// Returns: map of arXiv ID → citation count
func querySemantic(client *http.Client, arxivIDs []string) (map[string]int, error) {
	results := make(map[string]int)

	// Build the batch request body
	// Semantic Scholar accepts "ARXIV:{id}" format
	ids := make([]string, len(arxivIDs))
	idMap := make(map[string]string) // normalized → original
	for i, id := range arxivIDs {
		ids[i] = fmt.Sprintf("ARXIV:%s", id)
		idMap[strings.ToLower(id)] = id
	}

	body, _ := json.Marshal(map[string]interface{}{
		"ids": ids,
	})

	reqURL := "https://api.semanticscholar.org/graph/v1/paper/batch?fields=externalIds,citationCount"
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody[:min(200, len(respBody))]))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Response is an array — some entries can be null (paper not found)
	var papers []*s2Paper
	if err := json.Unmarshal(respBody, &papers); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	for _, p := range papers {
		if p == nil {
			continue
		}
		// Match back to original arXiv ID
		if p.ExternalIDs != nil && p.ExternalIDs.ArXiv != "" {
			origID, ok := idMap[strings.ToLower(p.ExternalIDs.ArXiv)]
			if ok {
				results[origID] = p.CitationCount
			}
		}
	}

	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
