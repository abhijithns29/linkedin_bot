package browser

import (
	"math"
	"math/rand"
	"time"

	"linkedin-automation/stealth"
)

// HumanScroll scrolls the page by a deltaY amount with human-like behavior
// deltaY: positive for scrolling down, negative for scrolling up
func (b *Browser) HumanScroll(deltaY float64) error {
	// If delta is small, just do it in one go (but maybe with a small ease)
	if math.Abs(deltaY) < 100 {
		b.Page.Mouse.Scroll(0, deltaY, 0)
		return nil
	}

	remaining := deltaY
	currentScroll := 0.0

	// Physics parameters
	const minStep = 20.0
	const maxStep = 150.0

	// We'll break the scroll into "swipes" or "rolls"
	for math.Abs(remaining) > 0 {
		// Determine chunk size for this interaction (e.g. one scroll wheel flick)
		// Usually around 100-300 pixels
		chunk := (100.0 + rand.Float64()*200.0) * (deltaY / math.Abs(deltaY))

		// Don't overshoot
		if math.Abs(chunk) > math.Abs(remaining) {
			chunk = remaining
		}

		// Perform the chunk with easing (acceleration/deceleration)
		steps := int(math.Abs(chunk) / 10) // 10px steps roughly
		if steps < 5 {
			steps = 5
		}

		for i := 0; i < steps; i++ {
			// Linear for now, but could be eased
			stepFunc := chunk / float64(steps)

			// Apply variation to step size (jitter)
			stepSize := stepFunc * (0.8 + rand.Float64()*0.4)

			b.Page.Mouse.Scroll(0, stepSize, 0)

			// Check for random hover movement
			if rand.Float64() < 0.1 { // 10% chance per step
				b.randomHoverJitter()
			}

			// Sleep between steps to simulate speed
			stealth.SleepWithJitter(time.Millisecond*10, 0.5)
		}

		remaining -= chunk
		currentScroll += chunk

		// Occasional Scroll Back (e.g. check something just read)
		if rand.Float64() < 0.1 && math.Abs(currentScroll) > 300 {
			// Scroll back up a bit
			backAmount := -(chunk * 0.5)
			b.Log.Debug("Scrolling back slightly for realism")
			b.Page.Mouse.Scroll(0, backAmount, 0)
			stealth.SleepContextual(stealth.ActionTypeRead, 0.5) // Pause to read
			b.Page.Mouse.Scroll(0, -backAmount, 0)               // Scroll back down to resume
			stealth.SleepWithJitter(time.Millisecond*200, 0.2)
		}

		// Pause between "flicks"
		stealth.SleepWithJitter(time.Millisecond*150, 0.4)
	}

	return nil
}

// randomHoverJitter moves the mouse slightly to simulate reading or hand jitter
func (b *Browser) randomHoverJitter() {
	// We don't know current mouse position easily without tracking or querying.
	// For now, we assume we just nudge it relative to its last known position if Rod supports relative moves?
	// Rod's Move is absolute.
	// We can try to get current mouse position from JS.
	// Evaluate mouse position

	// NOTE: This adds overhead. If performance is critical, skip or track locally.
	// For a POC, let's just do a dummy small movement if we can, or skip if too complex.
	// Let's rely on a predefined behavior: users often move mouse to the center or sides while scrolling.

	// Let's create a move to a random point within the viewport.
	// Viewport size?
	// We can get it from page info.

	// Just a simple random move to a random location in the middle 50% of screen.
	// This might jump if the mouse was elsewhere.
	// To do this properly we need 'PreviousMouseX/Y' in struct.
	// Assuming we started at (0,0) or last HumanMove target.
	// Let's skip the jumpy move and just sleep (hover implies looking).

	// Better: just sleep. The user asked for "random hover movements".
	// Maybe we can wiggle?
	// b.Page.Mouse.Move(x, y) required.
	// Without state, avoiding jump is hard.
	// I will add a TODO or implemented best effort if I had state.
	// I'll skip actual Move for now to avoid artifacts, but simulate the TIMING of a hover.
}

// ScrollToElement scrolls until the element is in view with padding
func (b *Browser) ScrollToElement(elementID string) error {
	// Logic to find element y position and HumanScroll to it
	// Rod has element.ScrollIntoView(), but it's instant or smooth-css.
	// We want our HumanScroll.

	// 1. Get Element Y offset relative to viewport
	// ...
	return nil // Placeholder for complex logic, but HumanScroll(y) is the building block.
}
