package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Issue represents a single analysis issue from the Hub API.
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

// AnalysisClient handles communication with the Hub API.
type AnalysisClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewAnalysisClient creates a new analysis client with modern HTTP client configuration.
func NewAnalysisClient(baseURL, token string) *AnalysisClient {
	return &AnalysisClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchIssues retrieves analysis issues from the Hub API with proper error handling.
func (c *AnalysisClient) FetchIssues(ctx context.Context, appID string) ([]Issue, error) {
	url := fmt.Sprintf("%s/hub/applications/%s/analysis/issues", c.baseURL, appID)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	// Set headers
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tackle2-addon-kai/fetch-analysis")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	
	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	
	return issues, nil
}

func main() {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	
	// Get required environment variables
	baseURL := os.Getenv("HUB_BASE_URL")
	token := os.Getenv("HUB_TOKEN")
	appID := os.Getenv("APP_ID")

	if baseURL == "" || appID == "" {
		fmt.Fprintf(os.Stderr, "Error: HUB_BASE_URL and APP_ID environment variables are required\n")
		os.Exit(1)
	}

	// Create analysis client
	client := NewAnalysisClient(baseURL, token)

	// Fetch analysis issues
	issues, err := client.FetchIssues(ctx, appID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching analysis: %v\n", err)
		os.Exit(1)
	}

	// Output as pretty-printed JSON
	output, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}
