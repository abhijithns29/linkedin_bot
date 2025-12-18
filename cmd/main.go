package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"linkedin-automation/auth"
	"linkedin-automation/browser"
	"linkedin-automation/config"
	"linkedin-automation/connect"
	"linkedin-automation/logger"
	"linkedin-automation/messaging"
	"linkedin-automation/search"
	"linkedin-automation/storage"
)

func main() {
	// Flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	mode := flag.String("mode", "connect", "Mode: 'connect' (search & add) or 'message' (follow-up)")
	keywords := flag.String("keywords", "Software Engineer", "General search keywords")
	title := flag.String("title", "", "Job title to search for")
	company := flag.String("company", "", "Company to search for")
	location := flag.String("location", "", "Location to search for")
	maxPages := flag.Int("pages", 1, "Max search pages to scrape")
	flag.Parse()

	// 1. Initialize Logger
	log := logger.New()
	log.Info("Starting LinkedIn Automation Bot", "mode", *mode)

	// 0. Stealth Check: Business Hours
	if !IsBusinessHours() {
		log.Warn("Outside business hours (9AM-6PM). proceeding cautiously.")
	}

	// 2. Load Config
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		// Fallback for demo purposes if file missing, assuming Env vars or defaults
		log.Warn("Could not load config file, proceeding with defaults/env", "error", err)
		cfg = &config.Config{}
		// Re-trigger defaults if empty
		if cfg.Limits.DailyConnections == 0 {
			cfg.Limits.DailyConnections = 10
		}
	}

	// Validate essential config for running
	if cfg.LinkedIn.Username == "" && cfg.UserDataDir == "" {
		log.Error("Configuration error: Username or UserDataDir is required.")
		os.Exit(1)
	}

	// 3. Initialize Browser
	log.Info("Initializing Browser...")
	b, err := browser.New(cfg, log)
	if err != nil {
		log.Error("Failed to initialize browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	// 4. Initialize Auth & Login
	log.Info("Authenticating...")
	authenticator := auth.New(b, cfg, log)
	if err := authenticator.Login(); err != nil {
		log.Error("Authentication failed", "error", err)
		// Dump screenshot for debug
		b.Page.MustScreenshot("login_failed.png")
		os.Exit(1)
	}

	// 5. Initialize Storage
	store, err := storage.NewJSONStore("state.json")
	if err != nil {
		log.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// 6. Initialize Services
	searcher := search.New(b, log)
	connector := connect.New(b, log, cfg.Limits.DailyConnections)
	messenger := messaging.New(b, log, store)

	// Executive Switch based on Mode
	if *mode == "message" {
		log.Info("Starting Workflow: Check Connections & Message")
		RunFollowUpWorkflow(log, messenger, cfg, store)
	} else {
		log.Info("Starting Workflow: Search & Connect", "keywords", *keywords)
		RunConnectWorkflow(log, searcher, connector, store, keywords, title, company, location, maxPages, cfg)
	}

	log.Info("Workflow completed successfully")

	// Wait for user input for demo visibility
	fmt.Println("\n=== POC Demonstration Completed ===")
	fmt.Println("Press Enter to close the browser and exit...")
	fmt.Scanln()
}

func RunFollowUpWorkflow(log logger.Logger, messenger *messaging.Service, cfg *config.Config, store *storage.MemoryStore) {
	// 1. Detect New Connections
	connections, err := messenger.DetectNewConnections(20) // Check last 20
	if err != nil {
		log.Error("Failed to detect connections", "error", err)
		return
	}

	// 2. Iterate and Message
	msgTemplate := "Hi {{firstname}}, great to connect with you! I see we share similar interests in tech."
	processed := 0

	for _, url := range connections {
		if processed >= cfg.Limits.DailyMessages {
			log.Warn("Daily message limit reached")
			break
		}

		if store.IsMessaged(url) {
			continue
		}

		log.Info("Processing follow-up", "url", url)
		if err := messenger.SendFollowUp(url, msgTemplate); err != nil {
			log.Error("Failed to send message", "url", url, "error", err)
			continue
		}

		processed++
		// Delay
		delay := time.Duration(20+rand.Intn(40)) * time.Second
		log.Info("Sleeping before next message", "seconds", delay)
		PerformRandomStealth(messenger.Browser) // Add random hover
		time.Sleep(delay)
	}
}

func RunConnectWorkflow(log logger.Logger, searcher search.Finder, connector *connect.Service, store *storage.MemoryStore, kw, title, company, loc *string, pages *int, cfg *config.Config) {
	// Step A: Search
	criteria := search.Criteria{
		Keywords: *kw,
		Title:    *title,
		Company:  *company,
		Location: *loc,
	}
	profiles, err := searcher.SearchPeople(criteria, *pages)
	if err != nil {
		log.Error("Search failed", "error", err)
		os.Exit(1)
	}

	// Shuffle profiles to randomize order
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(profiles), func(i, j int) { profiles[i], profiles[j] = profiles[j], profiles[i] })

	log.Info("Search complete", "profiles_found", len(profiles))

	// Step B: Filter and Select ONE Random Candidate
	var candidates []string
	for _, url := range profiles {
		if !store.IsRequestSent(url) && !store.IsConnected(url) {
			candidates = append(candidates, url)
		}
	}

	if len(candidates) == 0 {
		log.Info("No new eligible profiles found to connect with.")
		return
	}

	log.Info("Found eligible profiles", "count", len(candidates))

	// Shuffle candidates
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })

	// Select the first one
	targetURL := candidates[0]
	log.Info("Randomly selected profile for connection", "url", targetURL)

	noteTemplate := "Hi {{name}}, I noticed your profile and would love to connect!"

	// Attempt Connection
	log.Info("Sending connection request...")
	err = connector.SendConnectionRequest(targetURL, noteTemplate)
	if err != nil {
		log.Error("Failed to send connection request", "url", targetURL, "error", err)
		// We do not exit here, just log. The function returns and demo finishes.
	} else {
		// Mark as sent
		store.SaveRequest(targetURL)
		log.Info("Connection request sent successfully! Exiting for POC safety.")
	}
}

// IsBusinessHours checks if current time is between 9 AM and 6 PM
func IsBusinessHours() bool {
	now := time.Now()
	hour := now.Hour()
	return hour >= 9 && hour < 18
}

// PerformRandomStealth performs random hover actions
func PerformRandomStealth(b *browser.Browser) {
	// Randomly decide to hover over something safe
	if rand.Float32() > 0.7 { // 30% chance
		// Find a safe element to hover (e.g., logo, nav)
		// We try a few generic safe selectors
		el, err := b.Page.Element("h1, .global-nav__content, img")
		if err == nil {
			b.HumanMove(el)
		}
	}
}
