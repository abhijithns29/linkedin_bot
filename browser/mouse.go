package browser

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// HumanMove moves the mouse to the center of the element with human-like behavior.
func (b *Browser) HumanMove(element *rod.Element) error {
	// Get element box
	box, err := element.Shape()
	if err != nil {
		return err
	}

	// Calculate target geometry (center + random offset within bounds)
	// We stay within 80% of the element width/height to be safe
	rect := box.Box()
	targetX := rect.X + rect.Width/2 + (rand.Float64()-0.5)*rect.Width*0.8
	targetY := rect.Y + rect.Height/2 + (rand.Float64()-0.5)*rect.Height*0.8

	// Get current mouse position (Rod keeps track of this)
	// If we haven't moved yet, Rod defaults to 0,0.
	// We can't easily get the "real" current position from Rod without tracking it ourselves or querying the browser (which is async).
	// For simplicity, we assume we track it or just let the first move jump (or start from 0,0).
	// Rod's Mouse struct doesn't expose X,Y publicly easily, but we can assume previous known position.
	// However, to make this robust, let's just assume we start from where we left off.

	// Since Rod's Move logic is internal, we will implement our own step-by-step move.
	// We need to move from "current" to "target".
	// Let's assume we can get the current position by asking the cursor directly via JS evaluation if needed,
	// but efficiently simulating the path is generating a series of points.

	// NOTE: Rod's page.Mouse.Move(x, y) simply updates the position.
	// We want to generate a path.

	startX := b.LastMouseX
	startY := b.LastMouseY

	err = b.moveMouseAlongPath(startX, startY, targetX, targetY)
	if err == nil {
		b.LastMouseX = targetX
		b.LastMouseY = targetY
	}
	return err
}

func (b *Browser) moveMouseAlongPath(startX, startY, endX, endY float64) error {
	// Bezier Control Points
	// P0 = (startX, startY)
	// P3 = (endX, endY)

	dist := math.Hypot(endX-startX, endY-startY)

	// Control points slightly randomised to create an arc
	// Variance depends on distance
	variance := dist * 0.2

	p1x := startX + (endX-startX)*0.3 + (rand.Float64()-0.5)*variance
	p1y := startY + (endY-startY)*0.3 + (rand.Float64()-0.5)*variance

	p2x := startX + (endX-startX)*0.7 + (rand.Float64()-0.5)*variance
	p2y := startY + (endY-startY)*0.7 + (rand.Float64()-0.5)*variance

	// Steps: more steps = smoother, but slower.
	// Establish steps based on distance and "speed"
	// Speed: pixels per second.
	speed := 800.0 + rand.Float64()*400.0 // 800-1200 px/s
	duration := dist / speed
	if duration < 0.1 {
		duration = 0.1
	}

	steps := int(duration * 60) // 60 FPS
	if steps < 10 {
		steps = 10
	}

	// Function to calculate point at t (0 <= t <= 1)
	cubicBezier := func(t float64) (float64, float64) {
		mt := 1 - t
		mt2 := mt * mt
		mt3 := mt2 * mt
		t2 := t * t
		t3 := t2 * t

		x := mt3*startX + 3*mt2*t*p1x + 3*mt*t2*p2x + t3*endX
		y := mt3*startY + 3*mt2*t*p1y + 3*mt*t2*p2y + t3*endY
		return x, y
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)

		// Ease function for time (starts slow, speeds up, slows down)
		// Simple ease-in-out cubic
		easeT := t // linear fallback
		if t < 0.5 {
			easeT = 4 * t * t * t
		} else {
			easeT = 1 - math.Pow(-2*t+2, 3)/2
		}

		// Add wobble/micro-corrections?

		x, y := cubicBezier(easeT)

		// Overshoot calculation could be done by extending the curve, OR adding a separate correction phase.
		// Detailed implementation of overshoot:
		// Modify the Target P3 slightly beyond expected, then do a small second curve back.
		// For simplicity, we'll stick to the bezier trace for now which naturally curves.

		// Move the mouse
		b.Page.Mouse.MoveTo(proto.Point{X: x, Y: y})

		// Sleep for the time slice
		time.Sleep(time.Duration(duration / float64(steps) * float64(time.Second)))
	}

	return nil
}

// ClickElement moves to the element naturally and clicks
func (b *Browser) ClickElement(selector string) error {
	el, err := b.Page.Element(selector)
	if err != nil {
		return err
	}

	if err := b.HumanMove(el); err != nil {
		return err
	}

	// Add delay before click
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	return b.Page.Mouse.Click(proto.InputMouseButtonLeft, 1)
}
