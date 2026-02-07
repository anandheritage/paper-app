// Harvester: Fetches paper metadata from arXiv's OAI-PMH endpoint
// and stores it in PostgreSQL. Supports incremental harvesting via checkpoints.
//
// Usage:
//   go run ./cmd/harvest --db=$DATABASE_URL --set=cs          # Harvest all CS papers
//   go run ./cmd/harvest --db=$DATABASE_URL                    # Harvest ALL papers
//   go run ./cmd/harvest --db=$DATABASE_URL --set=cs --resume  # Resume interrupted harvest
//
// The harvester follows arXiv's terms of use:
// - Uses OAI-PMH (the official bulk metadata access method)
// - Respects rate limits (1 request per 3 seconds)
// - Identifies itself with a User-Agent string
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paper-app/backend/internal/domain"
	"github.com/paper-app/backend/pkg/oaipmh"
)

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	setName := flag.String("set", "", "OAI-PMH set to harvest (e.g. cs, math, physics). Empty = all.")
	fromDate := flag.String("from", "", "Harvest from this datestamp (YYYY-MM-DD)")
	resume := flag.Bool("resume", false, "Resume from last checkpoint")
	batchSize := flag.Int("batch", 200, "DB insert batch size")
	maxRecords := flag.Int("max", 0, "Max records to harvest (0 = unlimited)")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://paper:paper@localhost:5432/paper?sslmode=disable"
	}

	log.Println("=== arXiv OAI-PMH Harvester ===")
	log.Printf("Set: %s | From: %s | Resume: %v | MaxRecords: %d", orDefault(*setName, "_all"), orDefault(*fromDate, "earliest"), *resume, *maxRecords)

	// Connect to PostgreSQL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("\nReceived shutdown signal, saving checkpoint...")
		cancel()
	}()

	// Create OAI-PMH client
	client := oaipmh.NewClient()

	// Load or create checkpoint
	checkpointSet := orDefault(*setName, "_all")
	checkpoint, err := loadCheckpoint(ctx, pool, checkpointSet)
	if err != nil {
		log.Fatalf("Failed to load checkpoint: %v", err)
	}

	// Build initial request params
	params := oaipmh.ListRecordsParams{
		MetadataPrefix: oaipmh.MetadataPrefixArXiv,
		Set:            *setName,
	}

	if *resume && checkpoint.ResumptionToken != "" {
		params.ResumptionToken = checkpoint.ResumptionToken
		log.Printf("Resuming from checkpoint: %d harvested, token: %s...", checkpoint.TotalHarvested, checkpoint.ResumptionToken[:min(50, len(checkpoint.ResumptionToken))])
	} else if *fromDate != "" {
		params.From = *fromDate
	} else if checkpoint.LastDatestamp != "" && !*resume {
		// Incremental harvest from last datestamp
		params.From = checkpoint.LastDatestamp
		log.Printf("Incremental harvest from datestamp: %s", params.From)
	}

	// Update checkpoint status
	updateCheckpointStatus(ctx, pool, checkpointSet, "running")

	// Harvest loop
	var (
		totalNew     int
		totalUpdated int
		totalSkipped int
		totalDeleted int
		pageCount    int
		paperBuf     []*domain.Paper
		startTime    = time.Now()
		lastLog      = time.Now()
		lastDatestamp string
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("Harvest interrupted by shutdown signal")
			goto done
		default:
		}

		result, err := client.ListRecords(params)
		if err != nil {
			if strings.Contains(err.Error(), "rate limited") || strings.Contains(err.Error(), "503") {
				log.Printf("Rate limited, waiting 30s...")
				time.Sleep(30 * time.Second)
				continue
			}
			log.Printf("ERROR: %v (retrying in 10s...)", err)
			time.Sleep(10 * time.Second)
			continue
		}

		pageCount++

		for _, hp := range result.Papers {
			if hp.IsDeleted {
				totalDeleted++
				continue
			}

			if hp.ArXivID == "" || hp.Title == "" {
				totalSkipped++
				continue
			}

			paper := convertToPaper(hp)
			paperBuf = append(paperBuf, paper)

			if hp.Datestamp > lastDatestamp {
				lastDatestamp = hp.Datestamp
			}

			// Flush batch
			if len(paperBuf) >= *batchSize {
				inserted, err := bulkUpsert(ctx, pool, paperBuf)
				if err != nil {
					log.Printf("ERROR inserting batch: %v", err)
				} else {
					totalNew += inserted
					totalUpdated += len(paperBuf) - inserted
				}
				paperBuf = paperBuf[:0]
			}

			if *maxRecords > 0 && (totalNew+totalUpdated+totalSkipped) >= *maxRecords {
				log.Printf("Reached max records limit (%d)", *maxRecords)
				goto done
			}
		}

		// Progress logging
		total := totalNew + totalUpdated + totalSkipped + totalDeleted
		if time.Since(lastLog) > 15*time.Second || result.ResumptionToken == "" {
			elapsed := time.Since(startTime)
			rate := float64(total) / elapsed.Seconds()
			log.Printf("Page %d | %d new, %d updated, %d skipped, %d deleted | %.0f rec/s | Size: %s | Token: %s",
				pageCount, totalNew, totalUpdated, totalSkipped, totalDeleted, rate,
				orDefault(result.CompleteSize, "?"),
				truncate(result.ResumptionToken, 40))
			lastLog = time.Now()
		}

		// Save checkpoint periodically
		if pageCount%5 == 0 {
			saveCheckpoint(ctx, pool, checkpointSet, lastDatestamp, result.ResumptionToken, int64(totalNew+totalUpdated))
		}

		// Check for end of harvest
		if result.ResumptionToken == "" {
			log.Println("No more resumption token — harvest complete!")
			break
		}

		// Next page
		params = oaipmh.ListRecordsParams{
			ResumptionToken: result.ResumptionToken,
		}
	}

done:
	// Flush remaining papers
	if len(paperBuf) > 0 {
		inserted, err := bulkUpsert(ctx, pool, paperBuf)
		if err != nil {
			log.Printf("ERROR inserting final batch: %v", err)
		} else {
			totalNew += inserted
			totalUpdated += len(paperBuf) - inserted
		}
	}

	// Save final checkpoint
	saveCheckpoint(ctx, pool, checkpointSet, lastDatestamp, "", int64(totalNew+totalUpdated))
	updateCheckpointStatus(ctx, pool, checkpointSet, "completed")

	elapsed := time.Since(startTime)
	log.Printf("=== Harvest Complete ===")
	log.Printf("Duration:     %s", elapsed.Round(time.Second))
	log.Printf("New papers:   %d", totalNew)
	log.Printf("Updated:      %d", totalUpdated)
	log.Printf("Skipped:      %d", totalSkipped)
	log.Printf("Deleted:      %d", totalDeleted)
	log.Printf("Pages:        %d", pageCount)
}

// ---------- Database operations ----------

func convertToPaper(hp *oaipmh.HarvestedPaper) *domain.Paper {
	authorsJSON, _ := json.Marshal(hp.Authors)

	var pubDate *time.Time
	if !hp.PublishedDate.IsZero() {
		pubDate = &hp.PublishedDate
	}

	return &domain.Paper{
		ID:              uuid.New(),
		ExternalID:      hp.ArXivID,
		Source:          "arxiv",
		Title:           hp.Title,
		Abstract:        hp.Abstract,
		Authors:         authorsJSON,
		PublishedDate:   pubDate,
		UpdatedDate:     hp.UpdatedDate,
		PDFURL:          fmt.Sprintf("https://arxiv.org/pdf/%s", hp.ArXivID),
		PrimaryCategory: hp.PrimaryCategory,
		Categories:      hp.Categories,
		DOI:             hp.DOI,
		JournalRef:      hp.JournalRef,
		Comments:        hp.Comments,
		License:         hp.License,
		CreatedAt:       time.Now(),
	}
}

func bulkUpsert(ctx context.Context, pool *pgxpool.Pool, papers []*domain.Paper) (int, error) {
	if len(papers) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, p := range papers {
		batch.Queue(`
			INSERT INTO papers (id, external_id, source, title, abstract, authors, published_date, updated_date,
				pdf_url, primary_category, categories, doi, journal_ref, comments, license, citation_count, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 0, $16)
			ON CONFLICT (external_id) DO UPDATE SET
				title = EXCLUDED.title,
				abstract = EXCLUDED.abstract,
				authors = EXCLUDED.authors,
				published_date = COALESCE(EXCLUDED.published_date, papers.published_date),
				updated_date = EXCLUDED.updated_date,
				pdf_url = EXCLUDED.pdf_url,
				primary_category = EXCLUDED.primary_category,
				categories = EXCLUDED.categories,
				doi = COALESCE(NULLIF(EXCLUDED.doi, ''), papers.doi),
				journal_ref = COALESCE(NULLIF(EXCLUDED.journal_ref, ''), papers.journal_ref),
				comments = COALESCE(NULLIF(EXCLUDED.comments, ''), papers.comments),
				license = COALESCE(NULLIF(EXCLUDED.license, ''), papers.license)
		`,
			p.ID, p.ExternalID, p.Source, p.Title, p.Abstract, p.Authors,
			p.PublishedDate, p.UpdatedDate, p.PDFURL, p.PrimaryCategory,
			p.Categories, p.DOI, p.JournalRef, p.Comments, p.License, p.CreatedAt,
		)
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	inserted := 0
	for range papers {
		ct, err := br.Exec()
		if err != nil {
			continue
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}

	return inserted, nil
}

// ---------- Checkpoint management ----------

type checkpoint struct {
	LastDatestamp    string
	ResumptionToken string
	TotalHarvested  int64
}

func loadCheckpoint(ctx context.Context, pool *pgxpool.Pool, setName string) (*checkpoint, error) {
	cp := &checkpoint{}
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(last_datestamp, ''), COALESCE(last_resumption_token, ''), COALESCE(total_harvested, 0)
		 FROM harvest_checkpoints WHERE set_name = $1`, setName,
	).Scan(&cp.LastDatestamp, &cp.ResumptionToken, &cp.TotalHarvested)

	if err != nil {
		// Table might not exist yet or no checkpoint
		return &checkpoint{}, nil
	}
	return cp, nil
}

func saveCheckpoint(ctx context.Context, pool *pgxpool.Pool, setName, lastDatestamp, token string, total int64) {
	_, err := pool.Exec(ctx, `
		INSERT INTO harvest_checkpoints (set_name, last_datestamp, last_resumption_token, total_harvested, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (set_name) DO UPDATE SET
			last_datestamp = EXCLUDED.last_datestamp,
			last_resumption_token = EXCLUDED.last_resumption_token,
			total_harvested = harvest_checkpoints.total_harvested + EXCLUDED.total_harvested,
			updated_at = NOW()
	`, setName, lastDatestamp, token, total)
	if err != nil {
		log.Printf("WARN: Failed to save checkpoint: %v", err)
	}
}

func updateCheckpointStatus(ctx context.Context, pool *pgxpool.Pool, setName, status string) {
	var timeCol string
	switch status {
	case "running":
		timeCol = "started_at"
	case "completed", "failed":
		timeCol = "completed_at"
	default:
		return
	}

	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO harvest_checkpoints (set_name, status, %s, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (set_name) DO UPDATE SET
			status = EXCLUDED.status,
			%s = NOW(),
			updated_at = NOW()
	`, timeCol, timeCol), setName, status)
	if err != nil {
		log.Printf("WARN: Failed to update checkpoint status: %v", err)
	}
}

// ---------- Helpers ----------

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
