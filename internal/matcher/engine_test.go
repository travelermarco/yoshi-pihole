package matcher

import (
	"testing"

	"github.com/miekg/dns"

	"yoshi-pihole/internal/db"
)

func TestEnginePrecedence(t *testing.T) {
	data := &db.Snapshot{
		ExactAllow: map[string]struct{}{
			"allowed-exact.example.com": {},
		},
		ExactDeny: map[string]string{
			"allowed-exact.example.com": "gravity", // present in both; exact-allow must win
			"blocked-exact.example.com": "gravity",
			"blacklisted.example.com":   "manual",
		},
		RegexAllow: []db.RawRegexRule{
			{ID: 1, Raw: `^allowed-regex\.example\.com$`},
		},
		RegexDeny: []db.RawRegexRule{
			{ID: 2, Raw: `^.*\.ads\.example\.com$`},
		},
	}

	e := New()
	e.Load(data)

	cases := []struct {
		domain      string
		wantBlocked bool
		wantSource  string
	}{
		{"allowed-exact.example.com", false, ""},
		{"allowed-regex.example.com", false, ""},
		{"blocked-exact.example.com", true, "gravity"},
		{"blacklisted.example.com", true, "manual"},
		{"tracker.ads.example.com", true, "regex"},
		{"totally-unrelated.example.com", false, ""},
	}

	for _, c := range cases {
		d := e.Evaluate(c.domain, dns.TypeA)
		if d.Blocked != c.wantBlocked {
			t.Errorf("Evaluate(%q).Blocked = %v, want %v", c.domain, d.Blocked, c.wantBlocked)
			continue
		}
		if c.wantBlocked && d.Source != c.wantSource {
			t.Errorf("Evaluate(%q).Source = %q, want %q", c.domain, d.Source, c.wantSource)
		}
	}
}

func TestEngineTrailingDotAndCase(t *testing.T) {
	data := &db.Snapshot{
		ExactDeny: map[string]string{"example.com": "gravity"},
	}
	e := New()
	e.Load(data)

	if !e.Evaluate("EXAMPLE.COM.", dns.TypeA).Blocked {
		t.Error("expected case-insensitive, trailing-dot-tolerant match to be blocked")
	}
}

func TestEngineInvalidRegexSkipped(t *testing.T) {
	data := &db.Snapshot{
		RegexDeny: []db.RawRegexRule{
			{ID: 1, Raw: `^(a+)\1$`}, // backreference: invalid under Go's RE2 engine
			{ID: 2, Raw: `^bad\.example\.com$`},
		},
	}
	e := New()
	e.Load(data) // must not panic despite the invalid rule

	if !e.Evaluate("bad.example.com", dns.TypeA).Blocked {
		t.Error("expected the valid regex rule to still be loaded and match")
	}
	if e.Stats().RegexDeny != 1 {
		t.Errorf("expected only the valid regex rule to be compiled, got %d rules", e.Stats().RegexDeny)
	}
}
