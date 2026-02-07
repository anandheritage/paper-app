// s2import downloads the Semantic Scholar bulk papers dataset and indexes arXiv papers
// directly into OpenSearch. No PostgreSQL involved — data goes straight to the search index.
//
// Usage:
//
//	s2import --api-key=YOUR_KEY --opensearch=http://localhost:9200 --recreate-index
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paper-app/backend/pkg/opensearch"
	"github.com/paper-app/backend/pkg/s2"
)

func main() {
	apiKey := flag.String("api-key", os.Getenv("S2_API_KEY"), "Semantic Scholar API key")
	osEndpoint := flag.String("opensearch", os.Getenv("OPENSEARCH_ENDPOINT"), "OpenSearch endpoint URL")
	osIndex := flag.String("index", "papers", "OpenSearch index name")
	arxivOnly := flag.Bool("arxiv-only", true, "Only import papers with arXiv IDs")
	recreate := flag.Bool("recreate-index", false, "Delete and recreate index before import")
	startFile := flag.Int("start-file", 0, "Resume from this file number (0-based)")
	batchSize := flag.Int("batch-size", 500, "Bulk index batch size")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("S2_API_KEY is required (set via flag or S2_API_KEY env var)")
	}
	if *osEndpoint == "" {
		log.Fatal("OPENSEARCH_ENDPOINT is required (set via flag or OPENSEARCH_ENDPOINT env var)")
	}

	s2Client := s2.NewClient(*apiKey)
	osClient := opensearch.NewClient(opensearch.Config{
		Endpoint: strings.TrimRight(*osEndpoint, "/"),
		Index:    *osIndex,
	})

	ctx := context.Background()

	// 1. Get latest release
	log.Println("Fetching latest S2 dataset release...")
	release, err := s2Client.GetLatestRelease(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest release: %v", err)
	}
	log.Printf("Using release: %s", release.ReleaseID)

	// 2. Get papers dataset download URLs
	log.Println("Fetching papers dataset file list...")
	dataset, err := s2Client.GetDataset(ctx, release.ReleaseID, "papers")
	if err != nil {
		log.Fatalf("Failed to get papers dataset: %v", err)
	}
	log.Printf("Papers dataset has %d files", len(dataset.Files))

	// 3. Setup OpenSearch index
	if *recreate {
		log.Println("Deleting existing index...")
		if err := osClient.DeleteIndex(ctx); err != nil {
			log.Printf("WARNING: Delete index failed: %v", err)
		}
	}
	log.Println("Creating index (if needed)...")
	if err := osClient.CreateIndex(ctx); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}

	// Check current doc count
	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Current index doc count: %d", count)
	}

	// 4. Process each dataset file
	totalPapers := 0
	totalErrors := 0
	importStart := time.Now()

	filterFn := func(p *s2.S2Paper) bool {
		if p.Title == "" {
			return false
		}
		if *arxivOnly && p.GetArXivID() == "" {
			return false
		}
		return true
	}

	for i, fileURL := range dataset.Files {
		if i < *startFile {
			continue
		}

		shortURL := fileURL
		if len(shortURL) > 80 {
			shortURL = shortURL[:40] + "..." + shortURL[len(shortURL)-35:]
		}
		log.Printf("\n=== File %d/%d ===", i+1, len(dataset.Files))
		log.Printf("URL: %s", shortURL)
		fileStart := time.Now()

		count, err := s2Client.StreamPapersFile(ctx, fileURL, *batchSize, filterFn, func(papers []s2.S2Paper) error {
			docs := make([]*opensearch.PaperDoc, 0, len(papers))
			for j := range papers {
				doc := convertS2Paper(&papers[j])
				if doc != nil {
					docs = append(docs, doc)
				}
			}
			if len(docs) > 0 {
				indexed, err := osClient.BulkIndex(ctx, docs)
				if err != nil {
					return err
				}
				if indexed < len(docs) {
					totalErrors += len(docs) - indexed
				}
			}
			return nil
		})

		elapsed := time.Since(fileStart)
		totalPapers += count
		log.Printf("File %d done: %d papers in %v (total: %d, errors: %d)",
			i+1, count, elapsed.Round(time.Second), totalPapers, totalErrors)

		if err != nil {
			log.Printf("WARNING: Error processing file %d: %v (continuing...)", i+1, err)
		}
	}

	totalElapsed := time.Since(importStart)
	log.Printf("\n========================================")
	log.Printf("Import complete!")
	log.Printf("Total papers indexed: %d", totalPapers)
	log.Printf("Total errors: %d", totalErrors)
	log.Printf("Total time: %v", totalElapsed.Round(time.Second))
	if totalElapsed.Seconds() > 0 {
		log.Printf("Rate: %.0f papers/sec", float64(totalPapers)/totalElapsed.Seconds())
	}
	log.Printf("========================================")

	// Final count
	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Final index doc count: %d", count)
	}
}

func convertS2Paper(p *s2.S2Paper) *opensearch.PaperDoc {
	id := strconv.Itoa(p.CorpusID)

	// Determine source and external ID
	arxivID := p.GetArXivID()
	source := "s2"
	externalID := id
	if arxivID != "" {
		source = "arxiv"
		externalID = arxivID
	}

	// Authors
	authors := make([]map[string]string, 0, len(p.Authors))
	for _, a := range p.Authors {
		author := map[string]string{"name": a.Name}
		if a.AuthorID != "" {
			author["authorId"] = a.AuthorID
		}
		authors = append(authors, author)
	}

	// Fields of study → categories
	var categories []string
	seen := map[string]bool{}
	for _, f := range p.S2FieldsOfStudy {
		if !seen[f.Category] {
			categories = append(categories, f.Category)
			seen[f.Category] = true
		}
	}
	var primaryCategory string
	if len(categories) > 0 {
		primaryCategory = categories[0]
	}

	// PDF URL
	pdfURL := ""
	if arxivID != "" {
		pdfURL = "https://arxiv.org/pdf/" + arxivID
	}

	// Published date
	var pubDate *string
	if p.PublicationDate != nil && *p.PublicationDate != "" {
		pubDate = p.PublicationDate
	}

	// Journal ref
	journalRef := ""
	if p.Journal != nil && p.Journal.Name != "" {
		journalRef = p.Journal.Name
	}

	// DOI
	doi := p.GetDOI()

	// Abstract
	abstract := ""
	if p.Abstract != nil {
		abstract = *p.Abstract
	}

	return &opensearch.PaperDoc{
		ID:                       id,
		ExternalID:               externalID,
		Source:                   source,
		Title:                    p.Title,
		Abstract:                 abstract,
		Authors:                  authors,
		PublishedDate:            pubDate,
		Year:                     p.Year,
		PDFURL:                   pdfURL,
		PrimaryCategory:          primaryCategory,
		Categories:               categories,
		DOI:                      doi,
		JournalRef:               journalRef,
		CitationCount:            p.CitationCount,
		ReferenceCount:           p.ReferenceCount,
		InfluentialCitationCount: p.InfluentialCitationCount,
		Venue:                    p.Venue,
		PublicationTypes:         p.PublicationTypes,
		S2URL:                    p.URL,
		IsOpenAccess:             p.IsOpenAccess,
	}
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
