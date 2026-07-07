// Package trayicon embeds the Yoshi Pi-hole status bar glyphs. Both are
// rendered from Apple's own SF Symbols ("shield.fill" / "shield.slash"),
// not drawn by hand — status bar icons are forced to a monochrome template
// by macOS, so using the same iconography the system itself uses (Wi-Fi,
// Bluetooth, etc.) is what actually reads clean at that size, rather than
// a custom shape or a detailed color emoji shrunk down.
package trayicon

import _ "embed"

//go:embed shield-active.png
var active []byte

//go:embed shield-paused.png
var paused []byte

// Active is the glyph shown while blocking is on.
func Active() []byte { return active }

// Paused is the glyph shown while blocking is temporarily disabled.
func Paused() []byte { return paused }
