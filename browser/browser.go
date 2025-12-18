package browser

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"

	"linkedin-automation/config"
	"linkedin-automation/logger"
	"linkedin-automation/utils"
)

// Browser wraps the Rod browser instance
type Browser struct {
	RodBrowser *rod.Browser
	Page       *rod.Page
	Log        logger.Logger
	Cfg        *config.Config
	LastMouseX float64
	LastMouseY float64
}

// New initializes a new Browser instance with stealth settings
func New(cfg *config.Config, log logger.Logger) (*Browser, error) {
	// 1. Lifecycle Management: Use custom launcher
	l := launcher.New().
		Headless(cfg.Headless).
		Devtools(true) // Open devtools by default for debugging if headful

	if cfg.UserDataDir != "" {
		l.UserDataDir(cfg.UserDataDir)
	}

	if cfg.ProxyURL != "" {
		l.Proxy(cfg.ProxyURL)
	}

	// 2. Headful mode is implied if Headless is false in config
	// The prompt requested 'Headful mode', so we assume config sets it, or we force it here?
	// We'll respect the config, but default to headful if not specified in a real app.
	// For this specific request "Headful mode" is requested as a default behavior for this setup.

	// 3. Custom User Agent & 4. Disable navigator.webdriver
	// Ideally we set these via the launcher arguments or on the page.
	// Rod stealth handles navigator.webdriver removal.
	// We can set a flag for User data dir to persist session.

	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(url).MustConnect()

	// Create a new page (or use the default one)
	// We'll use MustPage to get the initial page
	page := browser.MustPage()

	// 5. Random Viewport
	// Generate random dimensions between reasonable desktop sizes
	width := 1024 + rand.Intn(1920-1024)
	height := 768 + rand.Intn(1080-768)

	// Apply stealth
	// stealth.JS includes standard stealth scripts
	// We can also configure specific evasions if needed.
	// rod-stealth automatically handles navigator.webdriver and other common leaks.
	page.MustEvalOnNewDocument(stealth.JS)

	// Set User Agent Override if provided, otherwise Stealth might provide a default
	if cfg.UserAgent != "" {
		page.MustEvalOnNewDocument(fmt.Sprintf(`Object.defineProperty(navigator, 'userAgent', { get: () => "%s" })`, cfg.UserAgent))
	} else {
		// Fallback to a modern UA if none provided to avoid HeadlessChrome UA
		ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		page.MustEvalOnNewDocument(fmt.Sprintf(`Object.defineProperty(navigator, 'userAgent', { get: () => "%s" })`, ua))
	}

	// Set Viewport
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             width,
		Height:            height,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to set viewport: %w", err)
	}

	log.Info("Browser initialized", "width", width, "height", height, "headless", cfg.Headless)

	return &Browser{
		RodBrowser: browser,
		Page:       page,
		Log:        log,
		Cfg:        cfg,
	}, nil
}

// Close cleans up the browser resources
func (b *Browser) Close() error {
	return b.RodBrowser.Close()
}

// NavigateTo goes to a URL with retry logic
func (b *Browser) NavigateTo(url string) error {
	b.Log.Info("Navigating to", "url", url)

	op := func() error {
		return b.Page.Navigate(url)
	}

	// Retry up to 3 times with 2s initial backoff
	// 2s -> 4s -> 8s
	return utils.RetryWithBackoff(op, 3, 2*time.Second, 10*time.Second)
}
