// Package service holds small pieces of runtime state shared across the DNS
// resolver and the API, starting with the blocking on/off ("bedtime mode")
// switch. Pi-hole has no built-in scheduler — just a timed disable — so this
// mirrors that exactly rather than inventing a cron system.
package service

import (
	"sync/atomic"
	"time"
)

const disabledIndefinitely = -1

// Bedtime tracks whether blocking is currently disabled, optionally with an
// expiry after which it re-enables itself automatically.
type Bedtime struct {
	disabledUntil atomic.Int64 // 0 = enabled, -1 = disabled indefinitely, >0 = disabled until this unix time
}

func NewBedtime() *Bedtime { return &Bedtime{} }

// Disable turns off blocking. d == 0 disables indefinitely (until Enable is
// called); d > 0 disables for that duration and re-enables automatically.
func (b *Bedtime) Disable(d time.Duration) {
	if d <= 0 {
		b.disabledUntil.Store(disabledIndefinitely)
		return
	}
	b.disabledUntil.Store(time.Now().Add(d).Unix())
}

func (b *Bedtime) Enable() {
	b.disabledUntil.Store(0)
}

// IsDisabled reports whether blocking is currently off, transparently
// re-enabling once a timed disable has expired.
func (b *Bedtime) IsDisabled() bool {
	v := b.disabledUntil.Load()
	switch {
	case v == 0:
		return false
	case v == disabledIndefinitely:
		return true
	default:
		if time.Now().Unix() >= v {
			b.disabledUntil.CompareAndSwap(v, 0)
			return false
		}
		return true
	}
}

// Status reports the raw state for the API: disabled flag, and the unix
// timestamp it re-enables at (0 = not disabled or disabled indefinitely).
func (b *Bedtime) Status() (disabled bool, until int64) {
	v := b.disabledUntil.Load()
	if v == 0 {
		return false, 0
	}
	if v == disabledIndefinitely {
		return true, 0
	}
	if time.Now().Unix() >= v {
		return false, 0
	}
	return true, v
}
