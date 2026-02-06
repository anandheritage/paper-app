package pubmed

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paper-app/backend/internal/domain"
)

const (
	esearchURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi"
	efetchURL  = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type SearchResult struct {
	Papers       []*domain.Paper
	TotalResults int
}

// ESearch response types
type ESearchResult struct {
	XMLName xml.Name `xml:"eSearchResult"`
	Count   int      `xml:"Count"`
	IDList  IDList   `xml:"IdList"`
}

type IDList struct {
	IDs []string `xml:"Id"`
}

// EFetch response types
type PubmedArticleSet struct {
	XMLName  xml.Name        `xml:"PubmedArticleSet"`
	Articles []PubmedArticle `xml:"PubmedArticle"`
}

type PubmedArticle struct {
	MedlineCitation MedlineCitation `xml:"MedlineCitation"`
	PubmedData      PubmedData      `xml:"PubmedData"`
}

type MedlineCitation struct {
	PMID    PMID    `xml:"PMID"`
	Article Article `xml:"Article"`
}

type PMID struct {
	Value string `xml:",chardata"`
}

type Article struct {
	Journal         Journal         `xml:"Journal"`
	ArticleTitle    string          `xml:"ArticleTitle"`
	Abstract        Abstract        `xml:"Abstract"`
	AuthorList      AuthorList      `xml:"AuthorList"`
	ArticleDate     []ArticleDate   `xml:"ArticleDate"`
	ELocationIDList []ELocationID   `xml:"ELocationID"`
}

type Journal struct {
	Title   string      `xml:"Title"`
	PubDate JournalDate `xml:"JournalIssue>PubDate"`
}

type JournalDate struct {
	Year  string `xml:"Year"`
	Month string `xml:"Month"`
	Day   string `xml:"Day"`
}

type Abstract struct {
	AbstractTexts []AbstractText `xml:"AbstractText"`
}

type AbstractText struct {
	Label string `xml:"Label,attr"`
	Text  string `xml:",chardata"`
}

type AuthorList struct {
	Authors []PubmedAuthor `xml:"Author"`
}

type PubmedAuthor struct {
	LastName    string        `xml:"LastName"`
	ForeName    string        `xml:"ForeName"`
	Affiliation []string      `xml:"AffiliationInfo>Affiliation"`
}

type ArticleDate struct {
	Year  string `xml:"Year"`
	Month string `xml:"Month"`
	Day   string `xml:"Day"`
}

type ELocationID struct {
	EIdType string `xml:"EIdType,attr"`
	Value   string `xml:",chardata"`
}

type PubmedData struct {
	ArticleIDList ArticleIDList `xml:"ArticleIdList"`
}

type ArticleIDList struct {
	ArticleIDs []ArticleID `xml:"ArticleId"`
}

type ArticleID struct {
	IDType string `xml:"IdType,attr"`
	Value  string `xml:",chardata"`
}

func (c *Client) Search(query string, limit, offset int) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Step 1: ESearch to get PMIDs
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("term", query)
	params.Set("retstart", fmt.Sprintf("%d", offset))
	params.Set("retmax", fmt.Sprintf("%d", limit))
	params.Set("sort", "relevance")
	params.Set("retmode", "xml")

	searchURL := fmt.Sprintf("%s?%s", esearchURL, params.Encode())
	resp, err := c.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("pubmed esearch request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read esearch response: %w", err)
	}

	var searchResult ESearchResult
	if err := xml.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to parse esearch response: %w", err)
	}

	if len(searchResult.IDList.IDs) == 0 {
		return &SearchResult{
			Papers:       []*domain.Paper{},
			TotalResults: searchResult.Count,
		}, nil
	}

	// Step 2: EFetch to get article details
	papers, err := c.fetchArticles(searchResult.IDList.IDs)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Papers:       papers,
		TotalResults: searchResult.Count,
	}, nil
}

func (c *Client) GetPaper(pmid string) (*domain.Paper, error) {
	papers, err := c.fetchArticles([]string{pmid})
	if err != nil {
		return nil, err
	}
	if len(papers) == 0 {
		return nil, nil
	}
	return papers[0], nil
}

func (c *Client) fetchArticles(pmids []string) ([]*domain.Paper, error) {
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("id", strings.Join(pmids, ","))
	params.Set("retmode", "xml")
	params.Set("rettype", "abstract")

	fetchURL := fmt.Sprintf("%s?%s", efetchURL, params.Encode())
	resp, err := c.httpClient.Get(fetchURL)
	if err != nil {
		return nil, fmt.Errorf("pubmed efetch request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read efetch response: %w", err)
	}

	var articleSet PubmedArticleSet
	if err := xml.Unmarshal(body, &articleSet); err != nil {
		return nil, fmt.Errorf("failed to parse efetch response: %w", err)
	}

	papers := make([]*domain.Paper, 0, len(articleSet.Articles))
	for _, article := range articleSet.Articles {
		paper := articleToPaper(&article)
		if paper != nil {
			papers = append(papers, paper)
		}
	}

	return papers, nil
}

func articleToPaper(article *PubmedArticle) *domain.Paper {
	pmid := article.MedlineCitation.PMID.Value
	if pmid == "" {
		return nil
	}

	// Build abstract text
	var abstractParts []string
	for _, text := range article.MedlineCitation.Article.Abstract.AbstractTexts {
		if text.Label != "" {
			abstractParts = append(abstractParts, fmt.Sprintf("%s: %s", text.Label, text.Text))
		} else {
			abstractParts = append(abstractParts, text.Text)
		}
	}
	abstract := strings.Join(abstractParts, "\n\n")

	// Build authors
	authors := make([]domain.Author, 0, len(article.MedlineCitation.Article.AuthorList.Authors))
	for _, a := range article.MedlineCitation.Article.AuthorList.Authors {
		name := strings.TrimSpace(fmt.Sprintf("%s %s", a.ForeName, a.LastName))
		affiliation := ""
		if len(a.Affiliation) > 0 {
			affiliation = a.Affiliation[0]
		}
		authors = append(authors, domain.Author{
			Name:        name,
			Affiliation: affiliation,
		})
	}
	authorsJSON, _ := json.Marshal(authors)

	// Parse published date
	var publishedDate *time.Time
	pubDate := article.MedlineCitation.Article.Journal.PubDate
	if pubDate.Year != "" {
		dateStr := pubDate.Year
		format := "2006"
		if pubDate.Month != "" {
			dateStr += " " + pubDate.Month
			format += " Jan"
			if pubDate.Day != "" {
				dateStr += " " + pubDate.Day
				format += " 2"
			}
		}
		if t, err := time.Parse(format, dateStr); err == nil {
			publishedDate = &t
		}
	}

	// Find DOI and PMC ID
	var doi, pmcID string
	for _, id := range article.PubmedData.ArticleIDList.ArticleIDs {
		switch id.IDType {
		case "doi":
			doi = id.Value
		case "pmc":
			pmcID = id.Value
		}
	}

	// Build PDF URL (PubMed Central if available, otherwise link to article)
	pdfURL := ""
	if pmcID != "" {
		pdfURL = fmt.Sprintf("https://www.ncbi.nlm.nih.gov/pmc/articles/%s/pdf/", pmcID)
	} else if doi != "" {
		pdfURL = fmt.Sprintf("https://doi.org/%s", doi)
	}

	// Metadata
	metadata := map[string]interface{}{
		"journal": article.MedlineCitation.Article.Journal.Title,
	}
	if doi != "" {
		metadata["doi"] = doi
	}
	if pmcID != "" {
		metadata["pmc_id"] = pmcID
		metadata["html_url"] = fmt.Sprintf("https://www.ncbi.nlm.nih.gov/pmc/articles/%s/", pmcID)
	}
	metadataJSON, _ := json.Marshal(metadata)

	return &domain.Paper{
		ExternalID:    pmid,
		Source:        "pubmed",
		Title:         strings.TrimSpace(article.MedlineCitation.Article.ArticleTitle),
		Abstract:      abstract,
		Authors:       authorsJSON,
		PublishedDate: publishedDate,
		PDFURL:        pdfURL,
		Metadata:      metadataJSON,
	}
}
