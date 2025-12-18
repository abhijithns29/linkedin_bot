package messaging

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-rod/rod/lib/proto"

	"linkedin-automation/browser"
	"linkedin-automation/logger"
	"linkedin-automation/stealth"
	"linkedin-automation/storage"
)

// Service handles messaging operations
type Service struct {
	Browser *browser.Browser
	Log     logger.Logger
	Store   storage.DataStore // Use the interface from storage
}

// New creates a new Messaging Service
func New(b *browser.Browser, l logger.Logger, s storage.DataStore) *Service {
	return &Service{
		Browser: b,
		Log:     l,
		Store:   s,
	}
}

// DetectNewConnections scans the detailed connections page for recently added connections
func (s *Service) DetectNewConnections(maxToCheck int) ([]string, error) {
	s.Log.Info("Checking for new connections...")
	url := "https://www.linkedin.com/mynetwork/invite-connect/connections/"
	if err := s.Browser.NavigateTo(url); err != nil {
		return nil, err
	}

	// Wait for list to load
	stealth.SleepContextual(stealth.ActionTypeRead, 1.5)

	// Scroll to load some
	s.Browser.HumanScroll(500)

	// Select connection cards
	// Usually a list `ul` with items `li` containing links to profiles
	// Selector for link: .mn-connection-card__link

	var newConnections []string

	// Wait for elements
	elements, err := s.Browser.Page.Elements(".mn-connection-card__link")
	if err != nil {
		s.Log.Warn("No connections found or selector changed", "error", err)
		return nil, nil // Return empty, not error
	}

	count := 0
	for _, el := range elements {
		if count >= maxToCheck {
			break
		}

		href, err := el.Attribute("href")
		if err == nil && href != nil {
			val := *href
			// Clean URL
			if strings.Contains(val, "/in/") {
				clean := strings.Split(val, "?")[0]
				if !strings.HasPrefix(clean, "http") {
					clean = "https://www.linkedin.com" + clean
				}

				// Check if we already messaged this person (skip effectively?)
				// Or we just return all recent connections and let the caller decide
				// The prompt says "Detect newly accepted connections".
				// We'll return them all, filtering logic belongs in the workflow loop + storage check.
				newConnections = append(newConnections, clean)
				count++
			}
		}
	}

	s.Log.Info("Detected recent connections", "count", len(newConnections))
	return newConnections, nil
}

// SendFollowUp sends a message to a connection if not already sent
// SendFollowUp sends a message to a connection if not already sent
func (s *Service) SendFollowUp(profileURL string, template string) error {
	if s.Store.IsMessaged(profileURL) {
		s.Log.Info("Already messaged this profile, skipping", "url", profileURL)
		return nil
	}

	s.Log.Info("Visiting profile to message", "url", profileURL)
	if err := s.Browser.NavigateTo(profileURL); err != nil {
		return err
	}

	// Wait for load
	stealth.SleepContextual(stealth.ActionTypeRead, 1.0)

	// Check for "Message" button
	// Primary button usually "Message" for 1st degree connections
	msgBtn, err := s.Browser.Page.ElementX(`//button[contains(., "Message")]`)
	if err != nil {
		// Possibly in "More" menu? Or not connected.
		return fmt.Errorf("message button not found (not connected?): %w", err)
	}

	s.Log.Info("Clicking Message button")
	if err := s.Browser.HumanMove(msgBtn); err != nil {
		msgBtn.Click(proto.InputMouseButtonLeft, 1)
	} else {
		msgBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	// This usually opens a chat box (overlay) or goes to messaging page
	// We wait for the chat input area
	// Selector: .msg-form__contenteditable or role="textbox" inside msg container

	stealth.SleepContextual(stealth.ActionTypeThink, 1.0)

	// Focus the text box
	// We look for the active message text box. It is usually an editable div.
	selector := `div[role="textbox"][aria-label^="Write a message"]`
	inputBox, err := s.Browser.Page.Element(selector)
	if err != nil {
		// Try generic contenteditable
		selector = `.msg-form__contenteditable`
		inputBox, err = s.Browser.Page.Element(selector)
		if err != nil {
			return fmt.Errorf("message input box not found: %w", err)
		}
	}

	// Check history again (maybe scrape chat content?)
	// For now we assume HistoryTracker is sufficient.

	// Prepare Message
	// Extract basic info for template
	nameEl, err := s.Browser.Page.Element("h1")
	name := "there"
	if err == nil {
		name = nameEl.MustText()
	}
	// Split full name to get first name
	firstName := strings.Split(name, " ")[0]

	msg := strings.ReplaceAll(template, "{{firstname}}", firstName)
	msg = strings.ReplaceAll(msg, "{{name}}", name)

	s.Log.Info("Typing message")
	if err := s.Browser.HumanType(inputBox, msg); err != nil {
		return err
	}

	// Verify content? (skip for now)

	// Send
	// Usually invalid to just hit Enter in some cases (adds newline), checking for "Send" button is safer.
	// Button type=submit usually
	sendBtn, err := s.Browser.Page.Element(`button[type="submit"]`)
	if err != nil || !sendBtn.MustVisible() {
		// Try finding by text "Send" within the message form
		sendBtn, err = s.Browser.Page.ElementX(`//button[contains(., "Send")]`)
		if err != nil {
			return errors.New("send message button not found")
		}
	}

	stealth.SleepContextual(stealth.ActionTypeThink, 0.5)

	s.Log.Info("Sending message")
	// Make sure we are clicking the send button for the *active* chat
	// Rod Element finding finds first match. If multiple chats open?
	// We assume we just opened one.

	if err := s.Browser.HumanMove(sendBtn); err != nil {
		sendBtn.Click(proto.InputMouseButtonLeft, 1)
	} else {
		sendBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	// Mark as sent
	s.Store.SaveMessage(profileURL)
	s.Log.Info("Message sent successfully")

	return nil
}
