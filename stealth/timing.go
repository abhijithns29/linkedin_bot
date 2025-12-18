package stealth

import (
	"math/rand"
	"time"
)

// ActionType defines the context for the delay
type ActionType string

const (
	ActionTypeClick  ActionType = "click"
	ActionTypeType   ActionType = "type"
	ActionTypeRead   ActionType = "read"
	ActionTypeScroll ActionType = "scroll"
	ActionTypeThink  ActionType = "think"
)

// TimingConfig holds configuration for specific action delays
type TimingConfig struct {
	Min time.Duration
	Max time.Duration
}

// Default timings for various actions
var defaultTimings = map[ActionType]TimingConfig{
	ActionTypeClick:  {Min: 100 * time.Millisecond, Max: 300 * time.Millisecond},
	ActionTypeType:   {Min: 50 * time.Millisecond, Max: 150 * time.Millisecond}, // Per keystroke
	ActionTypeRead:   {Min: 2 * time.Second, Max: 5 * time.Second},
	ActionTypeScroll: {Min: 500 * time.Millisecond, Max: 1500 * time.Millisecond},
	ActionTypeThink:  {Min: 1 * time.Second, Max: 3 * time.Second},
}

// RandomDuration returns a random duration between min and max
func RandomDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	delta := max - min
	return min + time.Duration(rand.Int63n(int64(delta)))
}

// SleepRandom sleeps for a random duration between min and max
func SleepRandom(min, max time.Duration) {
	time.Sleep(RandomDuration(min, max))
}

// SleepWithJitter sleeps for a base duration with +/- deviation percentage
// deviation should be between 0.0 and 1.0 (e.g., 0.2 for 20%)
func SleepWithJitter(base time.Duration, deviation float64) {
	if deviation < 0 {
		deviation = 0
	}

	// Calculate variation range
	delta := time.Duration(float64(base) * deviation)
	min := base - delta
	max := base + delta

	if min < 0 {
		min = 0
	}

	SleepRandom(min, max)
}

// SleepContextual sleeps for a duration appropriate for the given action
// Uses a multiplication factor 'intensity' (default 1.0) to speed up (<1) or slow down (>1)
func SleepContextual(action ActionType, intensity float64) {
	config, ok := defaultTimings[action]
	if !ok {
		// Fallback if unknown action
		config = TimingConfig{Min: 500 * time.Millisecond, Max: 1000 * time.Millisecond}
	}

	min := time.Duration(float64(config.Min) * intensity)
	max := time.Duration(float64(config.Max) * intensity)

	SleepRandom(min, max)
}
