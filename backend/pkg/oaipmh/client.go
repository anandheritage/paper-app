// Package oaipmh implements a client for the OAI-PMH v2.0 protocol,
// specifically tailored for harvesting arXiv metadata.
// Reference: https://info.arxiv.org/help/oa/index.html
package oaipmh

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the arXiv OAI-PMH v2.0 endpoint (updated March 2025).
	DefaultBaseURL = "https://oaipmh.arxiv.org/oai"

	// MetadataPrefixArXiv returns structured arXiv-specific metadata
	// including separated author names, categories, and license info.
	MetadataPrefixArXiv = "arXiv"

	// MetadataPrefixDC is simple Dublin Core (less structured).
	MetadataPrefixDC = "oai_dc"
)

// Client interacts with an OAI-PMH endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
	rateLimit  time.Duration
	lastCall   time.Time
}

// NewClient creates a new OAI-PMH client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // OAI-PMH responses can be large
		},
		rateLimit: 3 * time.Second, // Polite harvesting: max 1 req / 3s
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option configures the OAI-PMH client.
type Option func(*Client)

func WithBaseURL(u string) Option       { return func(c *Client) { c.baseURL = u } }
func WithRateLimit(d time.Duration) Option { return func(c *Client) { c.rateLimit = d } }
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.httpClient = hc } }

// ---------- XML response types ----------

// OAIResponse is the top-level OAI-PMH response.
type OAIResponse struct {
	XMLName        xml.Name        `xml:"OAI-PMH"`
	ResponseDate   string          `xml:"responseDate"`
	ListRecords    *ListRecordsRes `xml:"ListRecords"`
	Error          *OAIError       `xml:"error"`
}

type OAIError struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

type ListRecordsRes struct {
	Records         []Record         `xml:"record"`
	ResumptionToken *ResumptionToken `xml:"resumptionToken"`
}

type ResumptionToken struct {
	Token          string `xml:",chardata"`
	CompleteSize   string `xml:"completeListSize,attr"`
	Cursor         string `xml:"cursor,attr"`
	ExpirationDate string `xml:"expirationDate,attr"`
}

type Record struct {
	Header   RecordHeader `xml:"header"`
	Metadata Metadata     `xml:"metadata"`
}

type RecordHeader struct {
	Identifier string   `xml:"identifier"`
	Datestamp  string   `xml:"datestamp"`
	SetSpec    []string `xml:"setSpec"`
	Status     string   `xml:"status,attr"` // "deleted" if removed
}

type Metadata struct {
	ArXiv ArXivMetadata `xml:"arXiv"`
}

// ArXivMetadata is the arXiv-specific metadata format.
type ArXivMetadata struct {
	XMLName    xml.Name      `xml:"arXiv"`
	ID         string        `xml:"id"`
	Created    string        `xml:"created"`
	Updated    string        `xml:"updated"`
	Authors    ArXivAuthors  `xml:"authors"`
	Title      string        `xml:"title"`
	Categories string        `xml:"categories"` // space-separated
	Comments   string        `xml:"comments"`
	JournalRef string        `xml:"journal-ref"`
	DOI        string        `xml:"doi"`
	License    string        `xml:"license"`
	Abstract   string        `xml:"abstract"`
	MSCClass   string        `xml:"msc-class"`
	ACMClass   string        `xml:"acm-class"`
}

type ArXivAuthors struct {
	Authors []ArXivAuthor `xml:"author"`
}

type ArXivAuthor struct {
	Keyname     string `xml:"keyname"`
	Forenames   string `xml:"forenames"`
	Suffix      string `xml:"suffix"`
	Affiliation string `xml:"affiliation"`
}

// ---------- Parsed record ----------

// HarvestedPaper is a parsed, clean representation of a harvested paper.
type HarvestedPaper struct {
	ArXivID         string
	Title           string
	Abstract        string
	Authors         []ParsedAuthor
	Categories      []string
	PrimaryCategory string
	PublishedDate   time.Time
	UpdatedDate     *time.Time
	DOI             string
	JournalRef      string
	Comments        string
	License         string
	Datestamp       string // OAI datestamp for incremental harvesting
	IsDeleted       bool
}

type ParsedAuthor struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation,omitempty"`
}

// ---------- Public API ----------

// ListRecordsParams are parameters for a ListRecords request.
type ListRecordsParams struct {
	MetadataPrefix  string // required: "arXiv" or "oai_dc"
	Set             string // optional: e.g., "cs", "math", "physics"
	From            string // optional: datestamp YYYY-MM-DD
	Until           string // optional: datestamp YYYY-MM-DD
	ResumptionToken string // for continuing a previous harvest
}

// ListRecordsResult contains one page of harvested records.
type ListRecordsResult struct {
	Papers          []*HarvestedPaper
	ResumptionToken string // empty = no more pages
	CompleteSize    string // total number of records (may be empty)
	ResponseDate    string
}

// ListRecords fetches one page of records from the OAI-PMH endpoint.
func (c *Client) ListRecords(params ListRecordsParams) (*ListRecordsResult, error) {
	c.respectRateLimit()

	u, err := c.buildURL(params)
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}

	log.Printf("[OAI-PMH] GET %s", u)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "dapapers-harvester/1.0 (https://dapapers.com)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
		// Respect Retry-After header
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("rate limited (HTTP %d), Retry-After: %s", resp.StatusCode, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(500, len(body))]))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var oaiResp OAIResponse
	if err := xml.Unmarshal(body, &oaiResp); err != nil {
		return nil, fmt.Errorf("parse XML: %w", err)
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("OAI-PMH error [%s]: %s", oaiResp.Error.Code, oaiResp.Error.Message)
	}

	if oaiResp.ListRecords == nil {
		return &ListRecordsResult{ResponseDate: oaiResp.ResponseDate}, nil
	}

	result := &ListRecordsResult{
		ResponseDate: oaiResp.ResponseDate,
	}

	if oaiResp.ListRecords.ResumptionToken != nil {
		result.ResumptionToken = strings.TrimSpace(oaiResp.ListRecords.ResumptionToken.Token)
		result.CompleteSize = oaiResp.ListRecords.ResumptionToken.CompleteSize
	}

	for _, rec := range oaiResp.ListRecords.Records {
		paper := parseRecord(rec)
		if paper != nil {
			result.Papers = append(result.Papers, paper)
		}
	}

	return result, nil
}

// ---------- Internal helpers ----------

func (c *Client) buildURL(params ListRecordsParams) (string, error) {
	q := url.Values{}

	if params.ResumptionToken != "" {
		// When using a resumption token, only verb + token are allowed
		q.Set("verb", "ListRecords")
		q.Set("resumptionToken", params.ResumptionToken)
		return c.baseURL + "?" + q.Encode(), nil
	}

	q.Set("verb", "ListRecords")
	if params.MetadataPrefix == "" {
		params.MetadataPrefix = MetadataPrefixArXiv
	}
	q.Set("metadataPrefix", params.MetadataPrefix)

	if params.Set != "" {
		q.Set("set", params.Set)
	}
	if params.From != "" {
		q.Set("from", params.From)
	}
	if params.Until != "" {
		q.Set("until", params.Until)
	}

	return c.baseURL + "?" + q.Encode(), nil
}

func (c *Client) respectRateLimit() {
	elapsed := time.Since(c.lastCall)
	if elapsed < c.rateLimit {
		time.Sleep(c.rateLimit - elapsed)
	}
	c.lastCall = time.Now()
}

func parseRecord(rec Record) *HarvestedPaper {
	paper := &HarvestedPaper{
		Datestamp:  rec.Header.Datestamp,
		IsDeleted: rec.Header.Status == "deleted",
	}

	if paper.IsDeleted {
		// Extract arXiv ID from OAI identifier: oai:arXiv.org:2301.12345
		paper.ArXivID = extractArXivID(rec.Header.Identifier)
		return paper
	}

	meta := rec.Metadata.ArXiv
	paper.ArXivID = meta.ID
	paper.Title = cleanText(meta.Title)
	paper.Abstract = cleanText(meta.Abstract)
	paper.DOI = strings.TrimSpace(meta.DOI)
	paper.JournalRef = strings.TrimSpace(meta.JournalRef)
	paper.Comments = strings.TrimSpace(meta.Comments)
	paper.License = strings.TrimSpace(meta.License)

	// Parse categories (space-separated in the arXiv format)
	if cats := strings.TrimSpace(meta.Categories); cats != "" {
		paper.Categories = strings.Fields(cats)
		if len(paper.Categories) > 0 {
			paper.PrimaryCategory = paper.Categories[0]
		}
	}

	// Parse authors
	for _, a := range meta.Authors.Authors {
		name := strings.TrimSpace(a.Forenames + " " + a.Keyname)
		if a.Suffix != "" {
			name += " " + a.Suffix
		}
		paper.Authors = append(paper.Authors, ParsedAuthor{
			Name:        strings.TrimSpace(name),
			Affiliation: strings.TrimSpace(a.Affiliation),
		})
	}

	// Parse dates
	if t, err := time.Parse("2006-01-02", meta.Created); err == nil {
		paper.PublishedDate = t
	}
	if meta.Updated != "" {
		if t, err := time.Parse("2006-01-02", meta.Updated); err == nil {
			paper.UpdatedDate = &t
		}
	}

	return paper
}

func extractArXivID(oaiIdentifier string) string {
	// Format: oai:arXiv.org:2301.12345
	parts := strings.SplitN(oaiIdentifier, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return oaiIdentifier
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	// Collapse multiple whitespace/newlines into single spaces
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
