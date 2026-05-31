package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Custom errors for validation
var (
	ErrMissingEnvVar = errors.New("required environment variable is missing")
	ErrInvalidAppID  = errors.New("invalid application ID")
	ErrInvalidURL    = errors.New("invalid URL format")
)

// Config holds the configuration for the fetch-analysis tool
type Config struct {
	BaseURL       string
	Token         string
	AppID         string
	LabelSelector string
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("%w: HUB_BASE_URL", ErrMissingEnvVar)
	}
	if c.AppID == "" {
		return fmt.Errorf("%w: APP_ID", ErrMissingEnvVar)
	}

	// Validate URL format
	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidURL, c.BaseURL)
	}

	// Validate AppID is numeric
	if _, err := strconv.Atoi(c.AppID); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidAppID, c.AppID)
	}

	return nil
}

// BuildURL constructs the API endpoint URL
func (c *Config) BuildURL() string {
	return fmt.Sprintf("%s/hub/applications/%s/analysis/issues",
		strings.TrimRight(c.BaseURL, "/"), c.AppID)
}

// Issue represents a single analysis issue from the Hub.
type Issue struct {
	RuleSet     string     `json:"ruleset"`
	Rule        string     `json:"rule"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Effort      int        `json:"effort"`
	Labels      []string   `json:"labels,omitempty"`
	Incidents   []Incident `json:"incidents"`
}

// Incident represents a specific occurrence of an issue.
type Incident struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	CodeSnip string `json:"codeSnip,omitempty"`
}

// Validate validates the Issue struct
func (i *Issue) Validate() error {
	if strings.TrimSpace(i.RuleSet) == "" {
		return fmt.Errorf("ruleset cannot be empty")
	}
	if strings.TrimSpace(i.Rule) == "" {
		return fmt.Errorf("rule cannot be empty")
	}
	return nil
}

// Validate validates the Incident struct
func (inc *Incident) Validate() error {
	if strings.TrimSpace(inc.File) == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	if strings.TrimSpace(inc.Message) == "" {
		return fmt.Errorf("message cannot be empty")
	}
	return nil
}

// HTTPClient interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// AnalysisFetcher handles fetching analysis data
type AnalysisFetcher struct {
	client HTTPClient
	config *Config
}

// NewAnalysisFetcher creates a new analysis fetcher
func NewAnalysisFetcher(config *Config) *AnalysisFetcher {
	return &AnalysisFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
	}
}

// FetchAnalysis fetches analysis issues from the Hub API
func (af *AnalysisFetcher) FetchAnalysis() ([]Issue, error) {
	if err := af.config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	url := af.config.BuildURL()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	if af.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+af.config.Token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fetch-analysis/1.0")

	resp, err := af.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching analysis: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hub returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	// Validate each issue
	for i, issue := range issues {
		if err := issue.Validate(); err != nil {
			return nil, fmt.Errorf("issue %d validation failed: %w", i, err)
		}
	}

	return issues, nil
}

func main() {
	config := &Config{
		BaseURL:       os.Getenv("HUB_BASE_URL"),
		Token:         os.Getenv("HUB_TOKEN"),
		AppID:         os.Getenv("APP_ID"),
		LabelSelector: os.Getenv("LABEL_SELECTOR"),
	}

	fetcher := NewAnalysisFetcher(config)

	issues, err := fetcher.FetchAnalysis()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Pretty-print the issues
	out, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}
