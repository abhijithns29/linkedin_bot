package browser

import (
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"linkedin-automation/stealth"
)

// HumanType types text into an element with human-like behavior
func (b *Browser) HumanType(element *rod.Element, text string) error {
	// Ensure element is focused (optional, but good practice)
	// element.Focus() // Rod's Input usually handles individual key events well, but let's assume focus is needed or already there.

	err := element.Focus()
	if err != nil {
		return err
	}

	// Configuration for typing
	typoRate := 0.05 // 5% chance of typo per character
	chars := []rune(text)

	for i := 0; i < len(chars); i++ {
		char := chars[i]

		// Check for typo
		if rand.Float64() < typoRate {
			// Simulate a typo
			wrongChar := pickWrongChar(char)

			// Type the wrong character
			b.Page.InsertText(string(wrongChar))

			// Realization delay (longer than normal keypress)
			stealth.SleepWithJitter(time.Millisecond*300, 0.3)

			// Backspace
			b.Page.Keyboard.Press(input.Backspace)

			// Correction delay
			stealth.SleepWithJitter(time.Millisecond*150, 0.2)

			// Retry the loop for this character is not needed because we just corrected back to state before typo
			// We now proceed to type the correct character in the main flow
		}

		// Type the correct character
		b.Page.InsertText(string(char))

		// Calculate delay
		// Base delay from stealth package
		stealth.SleepContextual(stealth.ActionTypeType, 1.0)

		// Additional rhythm logic
		if char == ' ' {
			// Pause slightly more between words
			stealth.SleepWithJitter(time.Millisecond*100, 0.2)
		}
	}

	return nil
}

// pickWrongChar helps simulate immediate adjacency errors or random errors
func pickWrongChar(correct rune) rune {
	// Simple pool of common chars, in a real app this could be a QWERTY adjacency map
	// For POC, we just return a random lowercase letter or number if it's alphanumeric
	const alphanum = "abcdefghijklmnopqrstuvwxyz0123456789"

	// Just pick a random one that isn't the correct one
	for {
		r := rune(alphanum[rand.Intn(len(alphanum))])
		if r != correct {
			return r
		}
	}
}

// TypeInto finds an element and types the text using human-like behavior
func (b *Browser) TypeInto(selector, text string) error {
	el, err := b.Page.Element(selector)
	if err != nil {
		return err
	}

	// Maybe convert to human move first?
	if err := b.HumanMove(el); err != nil {
		return err
	}

	// Click to ensure focus
	if err := b.Page.Mouse.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}

	return b.HumanType(el, text)
}
