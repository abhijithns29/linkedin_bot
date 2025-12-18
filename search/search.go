package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"

	"linkedin-automation/browser"
	"linkedin-automation/logger"
	"linkedin-automation/stealth"
)

// Criteria defines the search filters
type Criteria struct {
	Keywords string
	Title    string
	Company  string
	Location string
}

// Finder defines the interface for searching
type Finder interface {
	SearchPeople(criteria Criteria, maxPages int) ([]string, error)
}

// Service implements Finder and handles search operations
type Service struct {
	Browser *browser.Browser
	Log     logger.Logger
}

// New creates a new Search Service
func New(b *browser.Browser, l logger.Logger) *Service {
	return &Service{
		Browser: b,
		Log:     l,
	}
}

// SearchPeople performs a search and scrapes profile URLs
func (s *Service) SearchPeople(criteria Criteria, maxPages int) ([]string, error) {
	// 1. Navigate to Search Page
	// Construct the query string based on criteria
	// We use the "keywords" parameter with boolean operators for simplicity: "Keywords AND Title AND Company..."
	// Or we can just join them with spaces which implies AND/OR depending on LI logic, usually good enough.
	// A better approach for specific fields is using the advanced filters if possible, but URL params for that are complex (e.g. &title=... is not always standard, often encoded filters).
	// For robust "v1" implementation, we'll build a rich keywords string.

	var parts []string
	if criteria.Keywords != "" {
		parts = append(parts, criteria.Keywords)
	}
	if criteria.Title != "" {
		parts = append(parts, criteria.Title)
	}
	if criteria.Company != "" {
		parts = append(parts, criteria.Company)
	}
	if criteria.Location != "" {
		parts = append(parts, criteria.Location)
	}

	fullQuery := strings.Join(parts, " ")
	safeQuery := strings.ReplaceAll(fullQuery, " ", "%20")
	searchURL := fmt.Sprintf("https://www.linkedin.com/search/results/people/?keywords=%s", safeQuery)

	s.Log.Info("Navigating to search", "url", searchURL)
	if err := s.Browser.NavigateTo(searchURL); err != nil {
		return nil, fmt.Errorf("failed to navigate to search: %w", err)
	}

	// Wait for results to load
	// Selector for result list container: .reusable-search__result-container
	// Increased timeout to 45 seconds for slow networks/checking
	// Also use Race to wait for either results OR "No results found"
	s.Log.Info("Waiting for search results...")

	// Just wait for the main list or a no-results indicator
	// .reusable-search__result-container is standard
	// .search-results-container is another potential
	// Just wait for the main list or a no-results indicator
	// .reusable-search__result-container is standard
	// .search-results-container is another potential
	// We use a shorter timeout for the check, and if it fails, we proceed to scrape anyway (might be slow load)
	// Wait for any link containing /in/ (profile links) as the sign of results loaded
	// This is generic and works regardless of container class changes
	err := s.Browser.Page.Timeout(30*time.Second).WaitElementsMoreThan("a[href*='/in/']", 2)
	if err != nil {
		s.Log.Warn("Search results selector timed out or not found, attempting to scrape anyway...", "error", err)
		s.Browser.Page.MustScreenshot("search_warning.png")
		// Do not return error, proceed to scraping logic which handles empty lists
	}

	uniqueURLs := make(map[string]bool)
	var results []string

	for page := 1; page <= maxPages; page++ {
		s.Log.Info("Scraping page", "page", page)

		// Human Scroll to load all lazy-loaded elements on the page
		// Scroll down in chunks to simulate reading/scanning
		for i := 0; i < 8; i++ {
			s.Browser.HumanScroll(400)
			stealth.SleepRandom(500*time.Millisecond, 1500*time.Millisecond)
		}

		// Extract Links
		// Select all anchor tags with /in/
		// Common selector: .app-aware-link
		elements, err := s.Browser.Page.Elements("a")
		if err == nil {
			for _, el := range elements {
				href, err := el.Attribute("href")
				if err == nil && href != nil {
					val := *href
					// Filter for profile links
					// We only check for /in/ and ensure it's not a mini-profile
					// We DO NOT filter out "linkedin.com/in/" because absolute URLs are valid common returns
					if strings.Contains(val, "/in/") && !strings.Contains(val, "/mini-profile/") {
						// linkedin.com/in/ check is to avoid dupes if href is absolute vs relative, usually it's relative or absolute.
						// Use simple check for /in/ standard pattern.

						// Clean URL (remove query params)
						cleanURL := strings.Split(val, "?")[0]

						// Ensure it's a full URL if relative
						if !strings.HasPrefix(cleanURL, "http") {
							cleanURL = "https://www.linkedin.com" + cleanURL
						}

						if !uniqueURLs[cleanURL] {
							uniqueURLs[cleanURL] = true
							results = append(results, cleanURL)
							s.Log.Debug("Found profile", "url", cleanURL)
						}
					}
				}
			}
		}

		s.Log.Info("Profiles found", "total_unique", len(results))

		// Pagination
		if page < maxPages {
			// Find "Next" button
			// Selector: button[aria-label="Next"] is very standard on LI

			// Allow time for "checking"
			stealth.SleepContextual(stealth.ActionTypeThink, 1.0)

			nextBtn, err := s.Browser.Page.Element(`button[aria-label="Next"]`)
			if err != nil {
				s.Log.Info("Next button not found, stopping pagination")
				break
			}

			// Check if disabled
			if disabled, _ := nextBtn.Attribute("disabled"); disabled != nil {
				s.Log.Info("Last page reached")
				break
			}

			// Scroll element into view if needed (HumanScroll logic handles generic scroll, but we need button visible)
			// Rod's ScrollIntoView works, but we want to be stealthy.
			// Ideally, we calculated Y pose. For now we assume bottom of page has the button.
			// We already scrolled down.

			s.Log.Info("Clicking next page")

			err = s.Browser.HumanMove(nextBtn)
			if err != nil {
				// Fallback
				nextBtn.ScrollIntoView()
				nextBtn.Click(proto.InputMouseButtonLeft, 1)
			} else {
				// Click with delay
				time.Sleep(time.Millisecond * 200)
				nextBtn.Click(proto.InputMouseButtonLeft, 1)
			}

			stealth.SleepContextual(stealth.ActionTypeRead, 1.5) // Wait for page load
		}
	}

	return results, nil
}
