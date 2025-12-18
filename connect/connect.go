package connect

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"linkedin-automation/browser"
	"linkedin-automation/logger"
	"linkedin-automation/stealth"
)

// Service handles connection requests
type Service struct {
	Browser    *browser.Browser
	Log        logger.Logger
	DailyLimit int
	sentCount  int
}

// New creates a new Connect Service
func New(b *browser.Browser, l logger.Logger, limit int) *Service {
	return &Service{
		Browser:    b,
		Log:        l,
		DailyLimit: limit,
		sentCount:  0,
	}
}

// SendConnectionRequest visits a profile and sends a request with a note
func (s *Service) SendConnectionRequest(profileURL string, messageTemplate string) error {
	if s.sentCount >= s.DailyLimit {
		return fmt.Errorf("daily connection limit reached (%d)", s.DailyLimit)
	}

	s.Log.Info("Visiting profile for connection", "url", profileURL)
	if err := s.Browser.NavigateTo(profileURL); err != nil {
		return err
	}

	// Wait for profile to load
	s.Log.Info("Waiting for profile content to load...")
	if el, err := s.Browser.Page.Timeout(15 * time.Second).Element("main"); err == nil {
		el.WaitVisible()
	} else {
		s.Log.Warn("Main profile content not found in time, trying to proceed anyway...")
	}

	// Extra wait for dynamic buttons
	stealth.SleepContextual(stealth.ActionTypeRead, 2.0)
	s.Browser.HumanScroll(300)

	// 0. Check for "Pending" status (already sent)
	if has, _, _ := s.Browser.Page.HasX(`//button[contains(., "Pending")]`); has {
		s.Log.Info("Connection already pending, skipping")
		return nil
	}

	// 1. Attempt to find "Connect" button
	// Strategy:
	// A. Primary action button (usually in the introduction/hero section)
	// B. "More" actions menu -> Connect option

	var connectBtn *rod.Element
	var err error

	// Try finding the primary Connect button first
	// We use a broader search first, then filter, or specific reliable selectors
	// Common Primary Buttons:
	// - button with text "Connect" (direct)
	// - aria-label="Connect"
	// We search specifically in the top card area (.pv-top-card or similar) to avoid nav bar
	s.Log.Debug("Looking for Connect button...")

	// 1. Attempt to find "Connect" button directly (Primary Action)
	// We only look for buttons that are strictly visible and main actions
	directConnectSelectors := []string{
		`//main//button[contains(@class, "artdeco-button--primary")][contains(., "Connect")]`,
		`//button[contains(@aria-label, "Connect")][not(contains(@aria-label, "Invite"))]`, // basic connect
	}

	s.Log.Debug("Checking for Direct Connect button...")
	for _, sel := range directConnectSelectors {
		btn, err := s.Browser.Page.Timeout(2 * time.Second).ElementX(sel)
		if err == nil {
			if visible, _ := btn.Visible(); visible {
				connectBtn = btn
				s.Log.Info("Found Direct Connect button", "selector", sel)
				break
			}
		}
	}

	// 2. If not found, Check "More" Menu for "Connect", "Add", or "Invite"
	if connectBtn == nil {
		s.Log.Debug("Direct Connect not found, checking 'More' menu")

		// Find More button
		// usually aria-label="More actions" within the top card
		moreBtn, err := s.Browser.Page.Timeout(2 * time.Second).ElementX(`//main//button[contains(@aria-label, "More actions")]`)
		if err != nil {
			// Fallback generic
			moreBtn, err = s.Browser.Page.Timeout(2 * time.Second).Element(`button[aria-label="More actions"]`)
		}

		if err == nil {
			s.Log.Info("Opening 'More' menu...")
			s.Browser.HumanMove(moreBtn)
			moreBtn.Click(proto.InputMouseButtonLeft, 1)
			stealth.SleepWithJitter(time.Second, 0.2)

			// Look for options INSIDE the menu
			// We look for text specifically because aria-labels might be complex
			menuOptions := []string{
				`//div[contains(@class, "artdeco-dropdown")]//span[text()="Connect"]`,
				`//div[contains(@class, "artdeco-dropdown")]//span[text()="Add"]`, // The screenshot showed "Add"
				`//div[contains(@class, "artdeco-dropdown")]//span[contains(text(), "Invite")]`,
				// Fallback generic role=button
				`//div[@role="button"]//span[text()="Connect"]`,
				`//div[@role="button"]//span[text()="Add"]`,
			}

			for _, sel := range menuOptions {
				if opt, err := s.Browser.Page.Timeout(2 * time.Second).ElementX(sel); err == nil {
					// It should be visible now
					if vis, _ := opt.Visible(); vis {
						connectBtn = opt
						s.Log.Info("Found Connect/Add option in More menu", "selector", sel)
						break
					}
				}
			}
		} else {
			s.Log.Warn("Could not find 'More' button")
		}
	}

	if connectBtn == nil {
		s.Log.Info("Connect button not found, attempting fallback to KEEP IN TOUCH (Follow/Message)")
		return s.tryFallbacks(profileURL, messageTemplate)
	}

	// Click Connect
	s.Log.Info("Clicking Connect button")
	// If it was found via span text, we might need to click its parent button?
	// Rod clicks the center of the element, so clicking the text span usually works if it captures events.
	if err := s.Browser.HumanMove(connectBtn); err != nil {
		connectBtn.Click(proto.InputMouseButtonLeft, 1)
	} else {
		connectBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	// 2. Handle Modal "You can customize this invitation"
	stealth.SleepContextual(stealth.ActionTypeThink, 0.8)

	// Check for "Weekly Limit Reached" or "Email Required"
	// Weekly limit modal text: "You've reached the weekly limit for connection requests"
	// Rod Page doesn't have Text(), check body
	pageText, _ := s.Browser.Page.MustElement("body").Text()
	if strings.Contains(pageText, "weekly limit") {
		s.Log.Error("Weekly connection limit reached! Stopping.")
		return fmt.Errorf("weekly connection limit reached")
	}

	if hasEmail, _, _ := s.Browser.Page.HasX(`//label[contains(., "Email")]`); hasEmail {
		s.Log.Warn("Email required for connection, skipping")
		s.Browser.Page.Keyboard.Press(input.Escape)
		return nil
	}

	// Check if the "Send" logic is blocked by "How do you know [Name]?"
	if strings.Contains(pageText, "How do you know") {
		s.Log.Warn("LinkedIn is asking 'How do you know this person', skipping strict verification")
		s.Browser.Page.Keyboard.Press(input.Escape)
		return nil
	}

	// 3. Add Note vs Direct Send
	// Look for "Add a note" button
	// We check for aria-label OR text content
	addNoteBtn, err := s.Browser.Page.ElementX(`//button[contains(@aria-label, "Add a note") or contains(., "Add a note")]`)
	if err == nil {
		s.Log.Info("Adding personalized note")
		addNoteBtn.Click(proto.InputMouseButtonLeft, 1)
		stealth.SleepWithJitter(time.Millisecond*500, 0.2)

		// Customize template
		nameEl, err := s.Browser.Page.Element("h1")
		name := "there"
		if err == nil {
			name = nameEl.MustText()
		}
		// First name only
		nameParts := strings.Split(name, " ")
		if len(nameParts) > 0 {
			name = nameParts[0]
		}

		msg := strings.ReplaceAll(messageTemplate, "{{name}}", name)

		// Type message
		textArea, err := s.Browser.Page.Element("textarea[name='message']")
		if err == nil {
			s.Browser.HumanType(textArea, msg)
		}
	} else {
		s.Log.Info("Add a note button not found, checking if we can just Send")
	}

	// 4. Click Send
	// Button usually says "Send" or "Send now"
	// Or "Send without a note" if we skipped note
	// We look for the primary action button in the modal dialog

	sendBtn, err := s.Browser.Page.Element(`button[aria-label="Send now"]`)
	if err != nil {
		// Try generic text "Send" inside the dialog
		// Dialog class usually .artdeco-modal or role="dialog"
		sendBtn, err = s.Browser.Page.ElementX(`//div[@role="dialog"]//button[contains(., "Send")]`)
		if err != nil {
			return errors.New("send button not found in dialog")
		}
	}

	s.Log.Info("Sending connection request")
	stealth.SleepContextual(stealth.ActionTypeThink, 0.5)

	if err := s.Browser.HumanMove(sendBtn); err != nil {
		sendBtn.Click(proto.InputMouseButtonLeft, 1)
	} else {
		sendBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	// Wait for modal to close to ensure it was sent
	time.Sleep(1 * time.Second)

	s.sentCount++
	s.Log.Info("Connection request sent", "count", s.sentCount, "limit", s.DailyLimit)

	return nil
}

// tryFallbacks attempts to Follow or Message if Connect fails
func (s *Service) tryFallbacks(url, msg string) error {
	// 1. Try FOLLOW
	s.Log.Info("Fallback: Checking for Follow button...")

	// Selectors for Follow (Direct OR Menu Item)
	followSelectors := []string{
		`//button[contains(@aria-label, "Follow")]`,
		`//button//span[text()="Follow"]`,
		// If inside the More menu (which might be open)
		`//div[contains(@class, "artdeco-dropdown")]//span[text()="Follow"]`,
		`//div[@role="button"]//span[text()="Follow"]`,
	}

	var followBtn *rod.Element
	// Direct check
	for _, sel := range followSelectors {
		if btn, err := s.Browser.Page.Timeout(2 * time.Second).ElementX(sel); err == nil {
			if vis, _ := btn.Visible(); vis {
				followBtn = btn
				break
			}
		}
	}

	// More Menu Check (if not found directly)
	if followBtn == nil {
		// We perform a simplified check inside More menu
		// Usually we need to open it again as it might be closed?
		// Since we don't track More menu state, we try to open it.
		// NOTE: Reuse logic or just try "Follow" inside likely open menu?
		// Safest is to try finding "Follow" in the DOM, if it's in the dropdown it might be visible now?
		// If the previous Connect check opened it, it might still be open.
		// Let's assume we need to find it.
		// For simplicity in this POC, we skip complex "More" re-opening for Follow to avoid UI flakiness.
		// We trust the direct check or assume if Connect wasn't in More, Follow might not be our priority.
		// User requirement "use follow button", usually visible.
	}

	if followBtn != nil {
		s.Log.Info("Clicking Follow button")
		s.Browser.HumanMove(followBtn)
		followBtn.Click(proto.InputMouseButtonLeft, 1)
		s.sentCount++ // Count as an interaction
		return nil
	}

	// 2. Try MESSAGE
	s.Log.Info("Fallback: Checking for Message button...")
	// Selectors for Message
	msgSelectors := []string{
		`//button[contains(@aria-label, "Message")]`,
		`//main//button[contains(., "Message")]`,
	}

	var msgBtn *rod.Element
	for _, sel := range msgSelectors {
		if btn, err := s.Browser.Page.Timeout(2 * time.Second).ElementX(sel); err == nil {
			if vis, _ := btn.Visible(); vis {
				msgBtn = btn
				break
			}
		}
	}

	if msgBtn != nil {
		s.Log.Info("Clicking Message button")
		s.Browser.HumanMove(msgBtn)
		msgBtn.Click(proto.InputMouseButtonLeft, 1)

		// Wait for Chat Window
		// usually div[role="textbox"] or .msg-form__contenteditable
		s.Log.Info("Waiting for chat window...")
		textBox, err := s.Browser.Page.Timeout(5 * time.Second).ElementX(`//div[@role="textbox"][@contenteditable="true"]`)
		if err == nil {
			s.Log.Info("Sending message via Message button")

			// Customize name
			// (Simplified for fallback)
			cleanMsg := strings.ReplaceAll(msg, "{{name}}", "there")

			s.Browser.HumanType(textBox, cleanMsg)
			stealth.SleepWithJitter(time.Second, 0.5)

			// Click Send
			// usually button[type="submit"] in the form
			if sendBtn, err := s.Browser.Page.Element(`button[type="submit"]`); err == nil {
				sendBtn.Click(proto.InputMouseButtonLeft, 1)
				s.sentCount++
				return nil
			}
		} else {
			s.Log.Warn("Chat window did not appear or locked (Premium).")
		}
	}

	return errors.New("no Connect, Follow, or Message options found")
}
