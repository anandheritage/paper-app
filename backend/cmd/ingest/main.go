// Package main provides a CLI tool to bulk-load arXiv paper metadata into
// PostgreSQL. It supports two input formats:
//
//   - JSON Lines (Kaggle format): one JSON object per line with fields
//     id, title, abstract, authors_parsed, categories, doi, versions, etc.
//
//   - Graph/edges JSON (Zenodo format): single JSON object with an "edges"
//     array where each element has {edge: "arXiv-id", attrs: {title, abstract, ...}}
//
// The format is auto-detected from the first byte of the file.
//
// Usage:
//
//	go run cmd/ingest/main.go \
//	  --file /path/to/arxiv-metadata.json \
//	  --db   "postgres://user:pass@host:5432/paper?sslmode=disable" \
//	  --batch 1000 \
//	  --categories "cs."   # optional: only ingest CS papers
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Kaggle JSON Lines format ───────────────────────────────────────────────

type kaggleRecord struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Abstract      string     `json:"abstract"`
	Categories    string     `json:"categories"` // space-separated
	Authors       string     `json:"authors"`
	AuthorsParsed [][]string `json:"authors_parsed"`
	DOI           string     `json:"doi"`
	JournalRef    string     `json:"journal-ref"`
	Comments      string     `json:"comments"`
	Versions      []struct {
		Version string `json:"version"`
		Created string `json:"created"`
	} `json:"versions"`
	UpdateDate string `json:"update_date"`
}

// ─── Zenodo graph/edges format ──────────────────────────────────────────────

type graphEdge struct {
	Edge  string    `json:"edge"` // arXiv ID
	Attrs edgeAttrs `json:"attrs"`
}

type edgeAttrs struct {
	Submitter  string   `json:"submitter"`
	Title      string   `json:"title"`
	Comments   string   `json:"comments"`
	Categories []string `json:"categories"`
	Abstract   string   `json:"abstract"`
	Date       string   `json:"date"` // "YYYY-MM-DD"
}

// ─── Common output ──────────────────────────────────────────────────────────

type author struct {
	Name string `json:"name"`
}

type paperRow struct {
	id            uuid.UUID
	externalID    string
	source        string
	title         string
	abstract      string
	authors       []byte // JSON
	publishedDate *time.Time
	pdfURL        string
	metadata      []byte // JSON
	citationCount int
	createdAt     time.Time
	categories    []string
}

// ─────────────────────────────────────────────────────────────────────────────

func main() {
	filePath := flag.String("file", "", "Path to arXiv metadata JSON file (required)")
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	batchSize := flag.Int("batch", 1000, "Records per INSERT batch")
	categoryPrefix := flag.String("categories", "", "Only ingest papers whose categories start with prefix (e.g. 'cs.')")
	limitRecords := flag.Int("limit", 0, "Max records to process (0 = all)")
	dropIndexes := flag.Bool("drop-indexes", true, "Drop GIN indexes before insert, recreate after")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("--file is required")
	}
	if *dbURL == "" {
		*dbURL = "postgres://paper:paper@localhost:5432/paper?sslmode=disable"
	}

	// Connect
	log.Println("Connecting to database...")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("DB ping failed: %v", err)
	}
	log.Println("Connected to PostgreSQL")
	ensureSchema(ctx, pool)

	if *dropIndexes {
		log.Println("Dropping GIN indexes for faster bulk insert...")
		dropGINIndexes(ctx, pool)
	}

	// Open file and detect format
	f, err := os.Open(*filePath)
	if err != nil {
		log.Fatalf("Cannot open file: %v", err)
	}
	defer f.Close()

	firstByte := make([]byte, 1)
	if _, err := f.Read(firstByte); err != nil {
		log.Fatalf("Cannot read file: %v", err)
	}
	f.Seek(0, io.SeekStart)

	isGraphFormat := firstByte[0] == '{'
	if isGraphFormat {
		log.Println("Detected: graph/edges JSON format (Zenodo)")
	} else {
		log.Println("Detected: JSON Lines format (Kaggle)")
	}

	// Ingestion loop
	var (
		batch     = &pgx.Batch{}
		batchN    int
		total     int
		inserted  int
		skipped   int
		filtered  int
		startTime = time.Now()
		lastLog   = time.Now()
	)

	insertSQL := `
		INSERT INTO papers (id, external_id, source, title, abstract, authors, published_date, pdf_url, metadata, citation_count, created_at, categories)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (external_id) DO NOTHING
	`

	process := func(p *paperRow) {
		if *limitRecords > 0 && total >= *limitRecords {
			return
		}
		if *categoryPrefix != "" {
			match := false
			for _, c := range p.categories {
				if strings.HasPrefix(c, *categoryPrefix) {
					match = true
					break
				}
			}
			if !match {
				filtered++
				return
			}
		}
		total++

		batch.Queue(insertSQL,
			p.id, p.externalID, p.source, p.title, p.abstract,
			p.authors, p.publishedDate, p.pdfURL, p.metadata,
			p.citationCount, p.createdAt, p.categories,
		)
		batchN++

		if batchN >= *batchSize {
			n := flushBatch(ctx, pool, batch, batchN)
			inserted += n
			batch = &pgx.Batch{}
			batchN = 0
		}

		if time.Since(lastLog) > 10*time.Second {
			elapsed := time.Since(startTime).Seconds()
			rate := float64(total) / elapsed
			log.Printf("%d processed, %d inserted, %d filtered | %.0f/sec",
				total, inserted, filtered, rate)
			lastLog = time.Now()
		}
	}

	if isGraphFormat {
		ingestGraph(f, process, *limitRecords)
	} else {
		ingestJSONL(f, process, *limitRecords)
	}

	// Flush remaining
	if batchN > 0 {
		n := flushBatch(ctx, pool, batch, batchN)
		inserted += n
	}

	elapsed := time.Since(startTime)
	log.Printf("=== Ingestion Complete ===")
	log.Printf("Processed: %d | Inserted: %d | Skipped: %d | Filtered: %d", total, inserted, skipped, filtered)
	log.Printf("Duration: %s | Rate: %.0f/sec", elapsed.Round(time.Second), float64(total)/elapsed.Seconds())

	if *dropIndexes {
		log.Println("Recreating indexes (may take a few minutes)...")
		createGINIndexes(ctx, pool)
	}
	log.Println("Running ANALYZE papers...")
	pool.Exec(ctx, "ANALYZE papers")
	log.Println("Done!")
}

// ─── Graph format ingestion (Zenodo) ────────────────────────────────────────

func ingestGraph(f *os.File, process func(*paperRow), limit int) {
	decoder := json.NewDecoder(f)

	// Read past tokens until we reach the "edges" array
	for {
		t, err := decoder.Token()
		if err != nil {
			log.Fatalf("Failed to read JSON token: %v", err)
		}
		if key, ok := t.(string); ok && key == "edges" {
			// Next token should be '['
			t2, err := decoder.Token()
			if err != nil {
				log.Fatalf("Expected '[' after edges: %v", err)
			}
			if delim, ok := t2.(json.Delim); !ok || delim != '[' {
				log.Fatalf("Expected '[', got %v", t2)
			}
			break
		}
	}

	count := 0
	for decoder.More() {
		if limit > 0 && count >= limit {
			break
		}
		var edge graphEdge
		if err := decoder.Decode(&edge); err != nil {
			log.Printf("WARN: decode edge: %v", err)
			continue
		}
		if edge.Edge == "" || edge.Attrs.Title == "" {
			continue
		}

		p := graphEdgeToPaper(&edge)
		process(p)
		count++
	}
}

func graphEdgeToPaper(e *graphEdge) *paperRow {
	a := e.Attrs

	// Author from submitter
	var authors []author
	if a.Submitter != "" {
		authors = append(authors, author{Name: strings.TrimSpace(a.Submitter)})
	}
	authorsJSON, _ := json.Marshal(authors)

	// Date
	var pubDate *time.Time
	if a.Date != "" {
		if t, err := time.Parse("2006-01-02", a.Date); err == nil {
			pubDate = &t
		}
	}

	// Categories
	categories := a.Categories

	// Metadata
	meta := map[string]interface{}{
		"html_url": fmt.Sprintf("https://arxiv.org/html/%s", e.Edge),
	}
	if a.Comments != "" {
		meta["comments"] = a.Comments
	}
	if len(categories) > 0 {
		meta["categories"] = categories
	}
	metaJSON, _ := json.Marshal(meta)

	title := strings.Join(strings.Fields(a.Title), " ")
	abstract := strings.TrimSpace(a.Abstract)
	abstract = strings.Join(strings.Fields(abstract), " ")

	return &paperRow{
		id:            uuid.New(),
		externalID:    e.Edge,
		source:        "arxiv",
		title:         title,
		abstract:      abstract,
		authors:       authorsJSON,
		publishedDate: pubDate,
		pdfURL:        fmt.Sprintf("https://arxiv.org/pdf/%s", e.Edge),
		metadata:      metaJSON,
		citationCount: 0,
		createdAt:     time.Now(),
		categories:    categories,
	}
}

// ─── JSON Lines format ingestion (Kaggle) ───────────────────────────────────

func ingestJSONL(f *os.File, process func(*paperRow), limit int) {
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	count := 0
	for scanner.Scan() {
		if limit > 0 && count >= limit {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec kaggleRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.ID == "" || rec.Title == "" {
			continue
		}
		p := kaggleRecordToPaper(&rec)
		process(p)
		count++
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func kaggleRecordToPaper(rec *kaggleRecord) *paperRow {
	var authors []author
	if len(rec.AuthorsParsed) > 0 {
		for _, parts := range rec.AuthorsParsed {
			if len(parts) >= 2 {
				last := strings.TrimSpace(parts[0])
				first := strings.TrimSpace(parts[1])
				name := strings.TrimSpace(first + " " + last)
				if name != "" {
					authors = append(authors, author{Name: name})
				}
			}
		}
	}
	authorsJSON, _ := json.Marshal(authors)

	var pubDate *time.Time
	if len(rec.Versions) > 0 && rec.Versions[0].Created != "" {
		for _, layout := range []string{
			"Mon, 2 Jan 2006 15:04:05 GMT",
			"Mon, 2 Jan 2006 15:04:05 MST",
			time.RFC1123,
		} {
			if t, err := time.Parse(layout, rec.Versions[0].Created); err == nil {
				pubDate = &t
				break
			}
		}
	}
	if pubDate == nil && rec.UpdateDate != "" {
		if t, err := time.Parse("2006-01-02", rec.UpdateDate); err == nil {
			pubDate = &t
		}
	}

	categories := strings.Fields(rec.Categories)

	meta := map[string]interface{}{
		"html_url": fmt.Sprintf("https://arxiv.org/html/%s", rec.ID),
	}
	if rec.DOI != "" {
		meta["doi"] = rec.DOI
	}
	if rec.JournalRef != "" {
		meta["journal_ref"] = rec.JournalRef
	}
	if rec.Comments != "" {
		meta["comments"] = rec.Comments
	}
	if len(categories) > 0 {
		meta["categories"] = categories
	}
	metaJSON, _ := json.Marshal(meta)

	title := strings.Join(strings.Fields(rec.Title), " ")
	abstract := strings.TrimSpace(rec.Abstract)
	abstract = strings.Join(strings.Fields(abstract), " ")

	return &paperRow{
		id:            uuid.New(),
		externalID:    rec.ID,
		source:        "arxiv",
		title:         title,
		abstract:      abstract,
		authors:       authorsJSON,
		publishedDate: pubDate,
		pdfURL:        fmt.Sprintf("https://arxiv.org/pdf/%s", rec.ID),
		metadata:      metaJSON,
		citationCount: 0,
		createdAt:     time.Now(),
		categories:    categories,
	}
}

// ─── Database helpers ───────────────────────────────────────────────────────

func flushBatch(ctx context.Context, pool *pgxpool.Pool, batch *pgx.Batch, n int) int {
	bCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	results := pool.SendBatch(bCtx, batch)
	defer results.Close()

	inserted := 0
	for i := 0; i < n; i++ {
		tag, err := results.Exec()
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate") {
				log.Printf("WARN: batch item %d: %v", i, err)
			}
			continue
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}
	return inserted
}

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) {
	pool.Exec(ctx, `ALTER TABLE papers ADD COLUMN IF NOT EXISTS categories TEXT[]`)
	pool.Exec(ctx, `ALTER TABLE papers ADD COLUMN IF NOT EXISTS citation_count INTEGER DEFAULT 0`)
}

func dropGINIndexes(ctx context.Context, pool *pgxpool.Pool) {
	for _, sql := range []string{
		"DROP INDEX IF EXISTS idx_papers_search_vector",
		"DROP INDEX IF EXISTS idx_papers_title_trgm",
		"DROP INDEX IF EXISTS idx_papers_categories",
	} {
		pool.Exec(ctx, sql)
	}
}

func createGINIndexes(ctx context.Context, pool *pgxpool.Pool) {
	indexes := []struct {
		name, sql string
	}{
		{"search_vector GIN", "CREATE INDEX IF NOT EXISTS idx_papers_search_vector ON papers USING GIN(search_vector)"},
		{"title trigram GIN", "CREATE INDEX IF NOT EXISTS idx_papers_title_trgm ON papers USING GIN(title gin_trgm_ops)"},
		{"categories GIN", "CREATE INDEX IF NOT EXISTS idx_papers_categories ON papers USING GIN(categories)"},
		{"published_date", "CREATE INDEX IF NOT EXISTS idx_papers_published_date ON papers(published_date DESC NULLS LAST)"},
		{"citation_count", "CREATE INDEX IF NOT EXISTS idx_papers_citation_count ON papers(citation_count DESC)"},
		{"source+date", "CREATE INDEX IF NOT EXISTS idx_papers_source_date ON papers(source, published_date DESC NULLS LAST)"},
		{"source+citations", "CREATE INDEX IF NOT EXISTS idx_papers_source_citations ON papers(source, citation_count DESC)"},
	}
	for _, idx := range indexes {
		start := time.Now()
		log.Printf("  Creating %s ...", idx.name)
		if _, err := pool.Exec(ctx, idx.sql); err != nil {
			log.Printf("  WARN: %s failed: %v", idx.name, err)
		} else {
			log.Printf("  %s done (%s)", idx.name, time.Since(start).Round(time.Millisecond))
		}
	}
}
