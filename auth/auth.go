package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"

	"linkedin-automation/browser"
	"linkedin-automation/config"
	"linkedin-automation/logger"
	"linkedin-automation/stealth"
)

// Authenticator handles login and session management
type Authenticator struct {
	Browser *browser.Browser
	Config  *config.Config
	Log     logger.Logger
}

// New creates a new Authenticator
func New(b *browser.Browser, cfg *config.Config, l logger.Logger) *Authenticator {
	return &Authenticator{
		Browser: b,
		Config:  cfg,
		Log:     l,
	}
}

// Login performs the login flow
func (a *Authenticator) Login() error {
	a.Log.Info("Checking login status...")

	// 1. Navigate to LinkedIn
	if err := a.Browser.NavigateTo("https://www.linkedin.com/feed/"); err != nil {
		return err
	}

	// 2. Check if already logged in
	// Look for search bar or nav elements
	// .global-nav__content is a good indicator
	loggedInSelector := ".global-nav__content"

	// Wait a bit to see if it loads
	// We use a short timeout because if not logged in, it redirects to login/home
	hasNav, _, _ := a.Browser.Page.Timeout(5 * time.Second).Has(loggedInSelector)
	if hasNav {
		a.Log.Info("Already logged in")
		return nil
	}

	a.Log.Info("Not logged in, attempting login flow")

	// If redirected to login page, good. If not, go there.
	// Check for username input
	// Check for username input (id="username" or name="session_key")
	if hasInput, _, _ := a.Browser.Page.Has("#username"); !hasInput {
		// Try fallback or navigate
		a.Log.Info("Navigating to login page")
		a.Browser.NavigateTo("https://www.linkedin.com/login")
	}

	// 3. Enter Credentials
	user := a.Config.LinkedIn.Username
	pass := a.Config.LinkedIn.Password

	if user == "" || pass == "" {
		return errors.New("cannot login: credentials missing in config/env")
	}

	// Username
	a.Log.Info("Entering username")
	// Wait for element
	userField, err := a.Browser.Page.Element("#username")
	if err != nil {
		// Fallback to name="session_key"
		userField, err = a.Browser.Page.Element(`input[name="session_key"]`)
		if err != nil {
			return errors.New("username field not found")
		}
	}
	// Wait for it to be visible
	if err := userField.WaitVisible(); err != nil {
		return fmt.Errorf("username field not visible: %w", err)
	}
	/*
		if err := userField.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
		if err := a.Browser.HumanType(userField, user); err != nil {
			return err
		}
	*/
	// Fallback to reliable input
	if err := userField.Input(user); err != nil {
		return err
	}

	// Password
	a.Log.Info("Entering password")
	passField, err := a.Browser.Page.Element("#password")
	if err != nil {
		// Fallback
		passField, err = a.Browser.Page.Element(`input[name="session_password"]`)
		if err != nil {
			return errors.New("password field not found")
		}
	}
	if err := passField.WaitVisible(); err != nil {
		return fmt.Errorf("password field not visible: %w", err)
	}
	/*
		if err := passField.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
		if err := a.Browser.HumanType(passField, pass); err != nil {
			return err
		}
	*/
	// Fallback to reliable input
	if err := passField.Input(pass); err != nil {
		return err
	}

	stealth.SleepContextual(stealth.ActionTypeThink, 0.5)

	// Remember me is usually checked by default or handled by UserDataDir persistence.

	// Click Sign In
	signInBtn, err := a.Browser.Page.Element(`button[type="submit"]`)
	if err != nil {
		return errors.New("sign in button not found")
	}

	a.Log.Info("Submitting login form")
	if err := a.Browser.HumanMove(signInBtn); err != nil {
		signInBtn.Click(proto.InputMouseButtonLeft, 1)
	} else {
		signInBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	// 4. Verification Check
	a.Log.Info("Waiting for navigation...")

	// Wait for either:
	// - Feed (success)
	// - Error message (failure)
	// - Challenge/Pin (2FA)

	// Use race to detect first match? Or simple sequence checks.
	// Ideally we wait for *any* of a set of selectors.
	// Rod Race matches first one that appears.

	feedSelector := ".global-nav__content"
	errorSelector := "#error-for-username, #error-for-password, .alert-content"
	// challengeSelector := "#app__container" -- removed unused

	// Simple polling loop for 30 seconds
	startTime := time.Now()
	for time.Since(startTime) < 30*time.Second {
		// Check Success
		if has, _, _ := a.Browser.Page.Has(feedSelector); has {
			a.Log.Info("Login successful (feed detected)")
			return nil
		}

		// Check Error
		if has, _, _ := a.Browser.Page.Has(errorSelector); has {
			// Extract error text
			el, _ := a.Browser.Page.Element(errorSelector)
			text := ""
			if el != nil {
				text = el.MustText()
			}
			a.Log.Error("Login failed with error", "message", text)
			return fmt.Errorf("login failed: %s", text)
		}

		// Check Challenge (Security Checkpoint)
		// Often checks for "Let's do a quick security check" text
		if strings.Contains(a.Browser.Page.MustInfo().Title, "Security Verification") ||
			strings.Contains(a.Browser.Page.MustInfo().Title, "Challenge") {
			a.Log.Warn("Security checkpoint/2FA detection! Manual intervention required.")
			return errors.New("manual intervention required: 2FA/checkpoint detected")
		}

		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("timeout waiting for login result")
}
