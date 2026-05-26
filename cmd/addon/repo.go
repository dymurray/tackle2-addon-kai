package main

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-hub/api"
)

// FetchRepository clones the application's SCM repository and returns the
// repository.SCM handle so callers can reuse it for Branch/Commit/push.
// It now uses improved error handling and modern Go patterns.
func FetchRepository(application *api.Application) (rp repository.SCM, err error) {
	if application == nil {
		return nil, errors.New("application cannot be nil")
	}
	
	if application.Repository == nil {
		return nil, errors.New("application repository not defined")
	}
	
	addon.Activity("Fetching repository for application ID %d", application.ID)
	addon.Activity("Repository URL: %s", application.Repository.URL)
	
	// Search for identity credentials with better error context
	var options []any
	identity, found, err := addon.Application.Identity(application.ID).Search().
		Direct("source").
		Indirect("source").
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to search for identity credentials: %w", err)
	}
	
	if found {
		options = append(options, identity)
		addon.Activity("Found identity credentials for repository access")
	} else {
		addon.Activity("No identity credentials found for repository access")
	}
	
	// Extract repository name with better handling
	repoName := extractRepoName(application.Repository.URL)
	SourceDir = path.Join(SourceDir, repoName)
	
	addon.Activity("Setting up repository in directory: %s", SourceDir)
	
	// Create repository instance with better error context
	rp, err = repository.New(
		SourceDir,
		application.Repository,
		options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository instance: %w", err)
	}
	
	addon.Activity("Fetching repository content...")
	
	// Fetch repository content with better error context
	err = rp.Fetch()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository content: %w", err)
	}
	
	addon.Activity("Repository fetched successfully to: %s", SourceDir)
	
	return rp, nil
}

// extractRepoName extracts a clean repository name from a URL.
// This provides better directory naming and handles edge cases.
func extractRepoName(url string) string {
	if url == "" {
		return "repo"
	}
	
	// Get the base name from the URL
	baseName := path.Base(url)
	
	// Handle URLs ending with .git
	if strings.HasSuffix(baseName, ".git") {
		baseName = strings.TrimSuffix(baseName, ".git")
	}
	
	// Split on dots and take the first part for better naming
	parts := strings.Split(baseName, ".")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	
	// Fallback for edge cases
	return "repo"
}
