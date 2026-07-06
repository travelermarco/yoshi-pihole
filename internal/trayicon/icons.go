// Package trayicon procedurally draws the two menu bar glyphs for the Yoshi
// Pi-hole status item (blocking active / blocking paused), so the app ships
// without external image assets. Both are template images: solid black
// shapes on a transparent background, which macOS automatically recolors to
// match the current menu bar (light or dark).
package trayicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

const size = 22 // standard macOS menu bar glyph size at @1x

// Active renders a solid shield — blocking is on.
func Active() []byte { return render(true) }

// Paused renders a hollow shield outline — blocking is paused/disabled.
func Paused() []byte { return render(false) }

func render(filled bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2-1
	outerRadius := float64(size)/2 - 3

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if !inShield(px, py, cx, cy, outerRadius) {
				continue
			}
			if filled {
				img.Set(x, y, color.NRGBA{0, 0, 0, 255})
				continue
			}
			// Hollow: only paint pixels within ~1.6px of the shield's edge.
			if nearShieldEdge(px, py, cx, cy, outerRadius) {
				img.Set(x, y, color.NRGBA{0, 0, 0, 255})
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// inShield approximates a shield silhouette: a rounded top (circle arc)
// tapering to a point at the bottom, which reads clearly as an ad-blocker
// glyph at menu bar size.
func inShield(px, py, cx, cy, r float64) bool {
	dx := px - cx
	top := cy - r*0.15
	if py <= top {
		// Rounded top half.
		dy := py - top
		return dx*dx+dy*dy <= r*r
	}
	// Tapering bottom half: width shrinks linearly to a point.
	bottom := cy + r*1.35
	if py > bottom {
		return false
	}
	frac := (py - top) / (bottom - top) // 0 at widest point, 1 at the tip
	widthAtY := r * (1 - frac)
	return math.Abs(dx) <= widthAtY
}

func nearShieldEdge(px, py, cx, cy, r float64) bool {
	const band = 1.6
	if !inShield(px, py, cx, cy, r) {
		return false
	}
	// A pixel is "near the edge" if moving outward by `band` in any of the
	// four cardinal directions falls outside the shield.
	return !inShield(px+band, py, cx, cy, r) ||
		!inShield(px-band, py, cx, cy, r) ||
		!inShield(px, py+band, cx, cy, r) ||
		!inShield(px, py-band, cx, cy, r)
}
