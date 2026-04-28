package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// fetch-analysis is a thin CLI that fetches analysis results from the
// Konveyor Hub API and prints them to stdout. It is designed to be called
// by an AI agent (via bash) during a migration run.
//
// Required environment variables:
//   HUB_BASE_URL - Hub API base URL (e.g. https://hub.example.com)
//   HUB_TOKEN    - Bearer token for Hub API auth
//   APP_ID       - Application ID to fetch analysis for
//
// Optional:
//   LABEL_SELECTOR - Filter analysis results by label

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

func main() {
	baseURL := os.Getenv("HUB_BASE_URL")
	token := os.Getenv("HUB_TOKEN")
	appID := os.Getenv("APP_ID")

	if baseURL == "" || appID == "" {
		fmt.Fprintf(os.Stderr, "HUB_BASE_URL and APP_ID environment variables are required\n")
		os.Exit(1)
	}

	// Build the request URL.
	// The Hub API exposes analysis issues at:
	//   GET /hub/applications/{id}/analysis/issues
	url := fmt.Sprintf("%s/hub/applications/%s/analysis/issues", strings.TrimRight(baseURL, "/"), appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating request: %v\n", err)
		os.Exit(1)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching analysis: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "hub returned %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading response: %v\n", err)
		os.Exit(1)
	}

	// Parse to validate and pretty-print.
	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		// If it doesn't match our struct, pass through the raw JSON.
		fmt.Println(string(body))
		return
	}

	out, _ := json.MarshalIndent(issues, "", "  ")
	fmt.Println(string(out))
}
