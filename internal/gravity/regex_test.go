package gravity

import (
	"testing"

	"github.com/miekg/dns"
)

func TestParseRegexBasic(t *testing.T) {
	rule, err := ParseRegex(`^ads\.example\.com$`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if !rule.Matches("ads.example.com", dns.TypeA) {
		t.Error("expected match on ads.example.com")
	}
	if rule.Matches("notads.example.com", dns.TypeA) {
		t.Error("did not expect match on notads.example.com")
	}
}

func TestParseRegexInlineComment(t *testing.T) {
	rule, err := ParseRegex(`^ads\.(?#tracking domain)example\.com$`)
	if err != nil {
		t.Fatalf("ParseRegex with inline comment: %v", err)
	}
	if !rule.Matches("ads.example.com", dns.TypeA) {
		t.Error("expected match after stripping inline comment")
	}
}

func TestParseRegexQueryTypeScoping(t *testing.T) {
	rule, err := ParseRegex(`^example\.com$;querytype=AAAA`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if rule.Matches("example.com", dns.TypeA) {
		t.Error("should not match A queries when scoped to AAAA")
	}
	if !rule.Matches("example.com", dns.TypeAAAA) {
		t.Error("should match AAAA queries when scoped to AAAA")
	}
}

func TestParseRegexQueryTypeNegated(t *testing.T) {
	rule, err := ParseRegex(`^example\.com$;querytype=!A`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if rule.Matches("example.com", dns.TypeA) {
		t.Error("negated querytype=!A should not match A queries")
	}
	if !rule.Matches("example.com", dns.TypeAAAA) {
		t.Error("negated querytype=!A should match AAAA queries")
	}
}

func TestParseRegexInvert(t *testing.T) {
	rule, err := ParseRegex(`^allowed\.example\.com$;invert`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if rule.Matches("allowed.example.com", dns.TypeA) {
		t.Error("invert should flip a match to non-match")
	}
	if !rule.Matches("other.example.com", dns.TypeA) {
		t.Error("invert should flip a non-match to match")
	}
}

func TestParseRegexReplyOverride(t *testing.T) {
	rule, err := ParseRegex(`^blocked\.example\.com$;reply=nxdomain`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if rule.Reply == nil || rule.Reply.Kind != "nxdomain" {
		t.Errorf("expected reply override nxdomain, got %+v", rule.Reply)
	}

	rule2, err := ParseRegex(`^blocked2\.example\.com$;reply=0.0.0.0`)
	if err != nil {
		t.Fatalf("ParseRegex: %v", err)
	}
	if rule2.Reply == nil || rule2.Reply.Kind != "ip" || rule2.Reply.IP != "0.0.0.0" {
		t.Errorf("expected reply override ip=0.0.0.0, got %+v", rule2.Reply)
	}
}

func TestParseRegexInvalidPattern(t *testing.T) {
	// RE2 (Go's regexp engine) does not support backreferences, unlike PCRE.
	_, err := ParseRegex(`^(a+)\1$`)
	if err == nil {
		t.Error("expected an error compiling a backreference pattern under RE2, got nil")
	}
}

func TestParseRegexUnknownExtension(t *testing.T) {
	_, err := ParseRegex(`^example\.com$;bogus=1`)
	if err == nil {
		t.Error("expected an error for an unknown regex extension")
	}
}
