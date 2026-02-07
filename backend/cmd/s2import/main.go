// s2import uses the Semantic Scholar Graph API (bulk search) to find arXiv papers
// and indexes them directly into OpenSearch. No API key required for basic usage.
//
// The tool runs multiple broad academic queries, paginates through all results,
// filters for papers with arXiv IDs, and bulk-indexes them into OpenSearch.
//
// Usage:
//
//	s2import --opensearch=http://localhost:9200 --recreate-index
//	s2import --api-key=KEY --opensearch=http://localhost:9200   # optional key for faster rate limits
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paper-app/backend/pkg/opensearch"
	"github.com/paper-app/backend/pkg/s2"
)

// broadQueries is a curated list of broad academic terms designed to maximize
// coverage of arXiv papers on Semantic Scholar. Each query can return up to
// 10M results, which we paginate through and filter for arXiv papers.
var broadQueries = []string{
	// Core CS/ML terms
	"deep learning",
	"neural network",
	"transformer",
	"reinforcement learning",
	"natural language processing",
	"computer vision",
	"generative adversarial",
	"graph neural",
	"convolutional neural",
	"recurrent neural",
	"attention mechanism",
	"machine learning",
	"representation learning",
	"federated learning",
	"transfer learning",
	"self-supervised",
	"contrastive learning",
	"diffusion model",
	"large language model",
	"foundation model",

	// AI/ML application terms
	"object detection",
	"image segmentation",
	"speech recognition",
	"text generation",
	"question answering",
	"sentiment analysis",
	"recommendation system",
	"anomaly detection",
	"time series",
	"knowledge graph",
	"point cloud",

	// Math/Theory
	"optimization algorithm",
	"stochastic gradient",
	"convex optimization",
	"variational inference",
	"Bayesian",
	"Monte Carlo",
	"differential equation",
	"algebraic geometry",
	"number theory",
	"topology",
	"combinatorics",
	"probability theory",
	"manifold",
	"dynamical system",
	"Markov chain",
	"Fourier transform",
	"partial differential",
	"linear algebra",
	"group theory",
	"category theory",

	// Physics
	"quantum computing",
	"quantum mechanics",
	"quantum field theory",
	"string theory",
	"dark matter",
	"gravitational wave",
	"condensed matter",
	"statistical mechanics",
	"particle physics",
	"cosmology",
	"general relativity",
	"superconductor",
	"black hole",
	"astrophysics",
	"plasma physics",
	"quantum entanglement",
	"lattice gauge",
	"renormalization",
	"Higgs boson",
	"neutrino",

	// More CS
	"distributed system",
	"blockchain",
	"cryptography",
	"compiler",
	"operating system",
	"database",
	"cloud computing",
	"edge computing",
	"parallel computing",
	"software engineering",
	"formal verification",
	"program synthesis",
	"robot",
	"autonomous driving",
	"multi-agent",

	// More broad terms
	"classification",
	"regression",
	"clustering",
	"dimensionality reduction",
	"embedding",
	"pretraining",
	"fine-tuning",
	"benchmark",
	"dataset",
	"survey",
	"simulation",
	"numerical method",
	"approximation",
	"convergence",
	"complexity",
	"entropy",
	"information theory",
	"signal processing",
	"control theory",
	"causal inference",
}

func main() {
	apiKey := flag.String("api-key", os.Getenv("S2_API_KEY"), "Semantic Scholar API key (optional, for higher rate limits)")
	osEndpoint := flag.String("opensearch", os.Getenv("OPENSEARCH_ENDPOINT"), "OpenSearch endpoint URL")
	osIndex := flag.String("index", "papers", "OpenSearch index name")
	recreate := flag.Bool("recreate-index", false, "Delete and recreate index before import")
	batchSize := flag.Int("batch-size", 500, "Bulk index batch size")
	startQuery := flag.Int("start-query", 0, "Resume from this query index (0-based)")
	maxPagesPerQuery := flag.Int("max-pages", 0, "Max pages per query (0=unlimited)")
	singleQuery := flag.String("query", "", "Run a single custom query instead of all broad queries")
	flag.Parse()

	if *osEndpoint == "" {
		log.Fatal("OPENSEARCH_ENDPOINT is required (set via flag or env var)")
	}

	if *apiKey != "" {
		log.Println("Using S2 API key for higher rate limits")
	} else {
		log.Println("No API key — using unauthenticated access (~1 req/sec rate limit)")
	}

	graphClient := s2.NewGraphClient(*apiKey)
	osClient := opensearch.NewClient(opensearch.Config{
		Endpoint: strings.TrimRight(*osEndpoint, "/"),
		Index:    *osIndex,
	})

	ctx := context.Background()

	// Setup OpenSearch index
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
	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Current index doc count: %d", count)
	}

	// Determine which queries to run
	queries := broadQueries
	if *singleQuery != "" {
		queries = []string{*singleQuery}
	}

	// Rate limit: ~1 req/sec without key, ~10 req/sec with key
	rateLimitDelay := 1100 * time.Millisecond
	if *apiKey != "" {
		rateLimitDelay = 150 * time.Millisecond
	}

	totalIndexed := 0
	totalSkipped := 0
	totalErrors := 0
	importStart := time.Now()

	for qi, query := range queries {
		if qi < *startQuery {
			continue
		}

		log.Printf("\n========== Query %d/%d: %q ==========", qi+1, len(queries), query)
		queryStart := time.Now()
		queryIndexed := 0
		queryScanned := 0
		token := ""
		page := 0

		for {
			if *maxPagesPerQuery > 0 && page >= *maxPagesPerQuery {
				log.Printf("  Hit max pages limit (%d), moving to next query", *maxPagesPerQuery)
				break
			}

			// Rate limiting
			time.Sleep(rateLimitDelay)

			result, err := graphClient.BulkSearch(ctx, query, token)
			if err != nil {
				if strings.Contains(err.Error(), "rate limited") {
					log.Printf("  Rate limited on page %d, waiting 10s...", page)
					time.Sleep(10 * time.Second)
					continue // Retry same page
				}
				log.Printf("  ERROR on page %d: %v (skipping rest of query)", page, err)
				totalErrors++
				break
			}

			if page == 0 {
				log.Printf("  Total matching papers: %d", result.Total)
			}

			// Filter and convert arXiv papers
			var docs []*opensearch.PaperDoc
			for i := range result.Data {
				queryScanned++
				p := &result.Data[i]
				arxivID := p.GetArXivID()
				if arxivID == "" {
					totalSkipped++
					continue
				}
				if p.Title == "" {
					continue
				}

				doc := convertGraphPaper(p)
				if doc != nil {
					docs = append(docs, doc)
				}
			}

			// Bulk index
			if len(docs) > 0 {
				// Index in sub-batches
				for start := 0; start < len(docs); start += *batchSize {
					end := start + *batchSize
					if end > len(docs) {
						end = len(docs)
					}
					indexed, err := osClient.BulkIndex(ctx, docs[start:end])
					if err != nil {
						log.Printf("  ERROR bulk indexing: %v", err)
						totalErrors += end - start
					} else {
						queryIndexed += indexed
						totalIndexed += indexed
						if indexed < end-start {
							totalErrors += (end - start) - indexed
						}
					}
				}
			}

			page++
			if page%10 == 0 {
				elapsed := time.Since(queryStart)
				log.Printf("  Page %d: scanned %d, indexed %d arXiv papers (%.0f/sec, %v elapsed)",
					page, queryScanned, queryIndexed, float64(queryIndexed)/elapsed.Seconds(), elapsed.Round(time.Second))
			}

			// Check for end of results
			if result.Token == "" || len(result.Data) == 0 {
				break
			}
			token = result.Token
		}

		queryElapsed := time.Since(queryStart)
		log.Printf("  Query %q done: %d pages, %d scanned, %d indexed (%v)",
			query, page, queryScanned, queryIndexed, queryElapsed.Round(time.Second))
	}

	totalElapsed := time.Since(importStart)
	log.Printf("\n========================================")
	log.Printf("Import complete!")
	log.Printf("Total arXiv papers indexed: %d", totalIndexed)
	log.Printf("Total non-arXiv skipped: %d", totalSkipped)
	log.Printf("Total errors: %d", totalErrors)
	log.Printf("Total time: %v", totalElapsed.Round(time.Second))
	if totalElapsed.Seconds() > 0 {
		log.Printf("Rate: %.0f papers/sec", float64(totalIndexed)/totalElapsed.Seconds())
	}
	log.Printf("========================================")

	// Final count
	if count, err := osClient.GetDocCount(ctx); err == nil {
		log.Printf("Final index doc count: %d", count)
	}
}

func convertGraphPaper(p *s2.GraphPaper) *opensearch.PaperDoc {
	arxivID := p.GetArXivID()
	if arxivID == "" {
		return nil
	}

	// Use corpusId as the document ID for deduplication
	id := strconv.Itoa(p.CorpusID)

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
	pdfURL := fmt.Sprintf("https://arxiv.org/pdf/%s", arxivID)
	if p.OpenAccessPdf != nil && p.OpenAccessPdf.URL != "" {
		pdfURL = p.OpenAccessPdf.URL
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

	// Abstract
	abstract := ""
	if p.Abstract != nil {
		abstract = *p.Abstract
	}

	// TLDR
	tldr := ""
	if p.TLDR != nil && p.TLDR.Text != "" {
		tldr = p.TLDR.Text
	}

	return &opensearch.PaperDoc{
		ID:                       id,
		ExternalID:               arxivID,
		Source:                   "arxiv",
		Title:                    p.Title,
		Abstract:                 abstract,
		Authors:                  authors,
		PublishedDate:            pubDate,
		Year:                     p.Year,
		PDFURL:                   pdfURL,
		PrimaryCategory:          primaryCategory,
		Categories:               categories,
		DOI:                      p.GetDOI(),
		JournalRef:               journalRef,
		CitationCount:            p.CitationCount,
		ReferenceCount:           p.ReferenceCount,
		InfluentialCitationCount: p.InfluentialCitationCount,
		Venue:                    p.Venue,
		PublicationTypes:         p.PublicationTypes,
		S2URL:                    p.URL,
		IsOpenAccess:             p.IsOpenAccess,
		TLDR:                     tldr,
	}
}
