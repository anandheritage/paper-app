// Indexer: Reads papers from PostgreSQL and indexes them into OpenSearch.
// Supports full re-indexing and incremental updates.
//
// Usage:
//   go run ./cmd/index --db=$DATABASE_URL --opensearch=$OPENSEARCH_URL
//   go run ./cmd/index --db=$DATABASE_URL --opensearch=$OPENSEARCH_URL --recreate  # Drop and recreate index
//   go run ./cmd/index --db=$DATABASE_URL --opensearch=$OPENSEARCH_URL --category=cs.AI  # Index specific category
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paper-app/backend/pkg/opensearch"
)

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	osURL := flag.String("opensearch", os.Getenv("OPENSEARCH_URL"), "OpenSearch endpoint URL")
	osIndex := flag.String("index", getEnvOrDefault("OPENSEARCH_INDEX", "papers"), "OpenSearch index name")
	osUser := flag.String("os-user", os.Getenv("OPENSEARCH_USER"), "OpenSearch username")
	osPass := flag.String("os-pass", os.Getenv("OPENSEARCH_PASS"), "OpenSearch password")
	recreate := flag.Bool("recreate", false, "Drop and recreate the index before indexing")
	batchSize := flag.Int("batch", 500, "Number of documents per bulk request")
	category := flag.String("category", "", "Only index papers with this primary category (e.g., cs.AI)")
	limit := flag.Int("limit", 0, "Max papers to index (0 = all)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://paper:paper@localhost:5432/paper?sslmode=disable"
	}
	if *osURL == "" {
		log.Fatal("OpenSearch URL is required (--opensearch or OPENSEARCH_URL)")
	}

	log.Println("=== Paper Indexer: PostgreSQL → OpenSearch ===")

	// Connect to PostgreSQL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Connect to OpenSearch
	osClient := opensearch.NewClient(opensearch.Config{
		Endpoint: strings.TrimRight(*osURL, "/"),
		Index:    *osIndex,
		Username: *osUser,
		Password: *osPass,
	})
	if err := osClient.Ping(ctx); err != nil {
		log.Fatalf("Failed to connect to OpenSearch: %v", err)
	}
	log.Println("Connected to OpenSearch")

	// Recreate index if requested
	if *recreate {
		log.Println("Deleting existing index...")
		if err := osClient.DeleteIndex(ctx); err != nil {
			log.Printf("WARN: Delete index: %v", err)
		}
	}

	log.Println("Creating index (if not exists)...")
	if err := osClient.CreateIndex(ctx); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("\nShutting down...")
		cancel()
	}()

	// Count papers to index
	countQuery := "SELECT COUNT(*) FROM papers WHERE title IS NOT NULL AND title != ''"
	args := []interface{}{}
	argIdx := 1
	if *category != "" {
		countQuery += " AND primary_category = $1"
		args = append(args, *category)
		argIdx++
	}

	var totalPapers int
	if err := pool.QueryRow(ctx, countQuery, args...).Scan(&totalPapers); err != nil {
		log.Fatalf("Failed to count papers: %v", err)
	}
	if *limit > 0 && *limit < totalPapers {
		totalPapers = *limit
	}
	log.Printf("Papers to index: %d", totalPapers)

	// Stream papers from PostgreSQL and bulk-index into OpenSearch
	selectQuery := `
		SELECT id, external_id, source, title, COALESCE(abstract, ''), authors,
			published_date, updated_date, pdf_url, COALESCE(primary_category, ''),
			categories, COALESCE(doi, ''), COALESCE(journal_ref, ''),
			COALESCE(comments, ''), COALESCE(license, '')
		FROM papers
		WHERE title IS NOT NULL AND title != ''
	`
	selectArgs := []interface{}{}
	if *category != "" {
		selectQuery += " AND primary_category = $1"
		selectArgs = append(selectArgs, *category)
	}
	selectQuery += " ORDER BY external_id"
	if *limit > 0 {
		if *category != "" {
			selectQuery += " LIMIT $2"
		} else {
			selectQuery += " LIMIT $1"
		}
		selectArgs = append(selectArgs, *limit)
	}
	_ = argIdx

	rows, err := pool.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		log.Fatalf("Failed to query papers: %v", err)
	}
	defer rows.Close()

	var (
		indexed   int
		errors    int
		batch     []*opensearch.PaperDoc
		startTime = time.Now()
		lastLog   = time.Now()
	)

	for rows.Next() {
		select {
		case <-ctx.Done():
			log.Println("Indexing interrupted")
			goto done
		default:
		}

		var (
			id              string
			externalID      string
			source          string
			title           string
			abstract        string
			authorsJSON     json.RawMessage
			publishedDate   *time.Time
			updatedDate     *time.Time
			pdfURL          string
			primaryCategory string
			categories      []string
			doi             string
			journalRef      string
			comments        string
			license         string
		)

		if err := rows.Scan(&id, &externalID, &source, &title, &abstract, &authorsJSON,
			&publishedDate, &updatedDate, &pdfURL, &primaryCategory,
			&categories, &doi, &journalRef, &comments, &license); err != nil {
			log.Printf("WARN: Scan error: %v", err)
			errors++
			continue
		}

		doc := &opensearch.PaperDoc{
			ID:              id,
			ExternalID:      externalID,
			Source:          source,
			Title:           title,
			Abstract:        abstract,
			PrimaryCategory: primaryCategory,
			Categories:      categories,
			DOI:             doi,
			JournalRef:      journalRef,
			PDFURL:          pdfURL,
		}

		// Parse authors for OpenSearch nested type
		var authors []map[string]string
		if err := json.Unmarshal(authorsJSON, &authors); err != nil {
			// Try array of objects with "name" field
			var authorObjs []struct {
				Name        string `json:"name"`
				Affiliation string `json:"affiliation"`
			}
			if err2 := json.Unmarshal(authorsJSON, &authorObjs); err2 == nil {
				for _, a := range authorObjs {
					authors = append(authors, map[string]string{"name": a.Name, "affiliation": a.Affiliation})
				}
			}
		}
		doc.Authors = authors

		if publishedDate != nil {
			d := publishedDate.Format("2006-01-02")
			doc.PublishedDate = &d
		}
		_ = updatedDate // UpdatedDate not in current PaperDoc; field kept for PG compat

		batch = append(batch, doc)

		if len(batch) >= *batchSize {
			n, err := osClient.BulkIndex(ctx, batch)
			if err != nil {
				log.Printf("ERROR: Bulk index failed: %v", err)
				errors += len(batch)
			} else {
				indexed += n
				errors += len(batch) - n
			}
			batch = batch[:0]

			if time.Since(lastLog) > 10*time.Second {
				elapsed := time.Since(startTime).Seconds()
				rate := float64(indexed) / elapsed
				pct := float64(indexed) / float64(totalPapers) * 100
				eta := time.Duration(float64(totalPapers-indexed)/rate) * time.Second
				log.Printf("Progress: %d/%d (%.1f%%) | %d errors | %.0f docs/s | ETA %s",
					indexed, totalPapers, pct, errors, rate, eta.Round(time.Second))
				lastLog = time.Now()
			}
		}
	}

done:
	// Flush remaining
	if len(batch) > 0 {
		n, err := osClient.BulkIndex(ctx, batch)
		if err != nil {
			log.Printf("ERROR: Final bulk index failed: %v", err)
			errors += len(batch)
		} else {
			indexed += n
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("=== Indexing Complete ===")
	log.Printf("Indexed:  %d documents", indexed)
	log.Printf("Errors:   %d", errors)
	log.Printf("Duration: %s", elapsed.Round(time.Second))
	log.Printf("Rate:     %.0f docs/sec", float64(indexed)/elapsed.Seconds())
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
