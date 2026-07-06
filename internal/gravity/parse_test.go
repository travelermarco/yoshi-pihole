package gravity

import (
	"reflect"
	"testing"
)

func TestParseHosts(t *testing.T) {
	body := `
# comment line
0.0.0.0 ads.example.com
127.0.0.1 tracker.example.com   # inline comment
0.0.0.0 localhost
0.0.0.0 broadcasthost
1.2.3.4 not-a-sinkhole.example.com
0.0.0.0 UPPER.Example.COM
`
	res := ParseHosts(body)
	want := []string{"ads.example.com", "tracker.example.com", "upper.example.com"}
	if !reflect.DeepEqual(res.Domains, want) {
		t.Errorf("ParseHosts domains = %v, want %v", res.Domains, want)
	}
}

func TestParsePlain(t *testing.T) {
	body := `
# comment
ads.example.com
tracker.example.net

not_a_domain
`
	res := ParsePlain(body)
	want := []string{"ads.example.com", "tracker.example.net"}
	if !reflect.DeepEqual(res.Domains, want) {
		t.Errorf("ParsePlain domains = %v, want %v", res.Domains, want)
	}
	if res.InvalidDomains != 1 {
		t.Errorf("ParsePlain InvalidDomains = %d, want 1 (not_a_domain has no dot)", res.InvalidDomains)
	}
}

func TestParseABP(t *testing.T) {
	body := `
! comment
[Adblock Plus 2.0]
||ads.example.com^
||tracker.example.com^$third-party
@@||exception.example.com^
##.cosmetic-rule
/path/based/rule.js
`
	res := ParseABP(body)
	want := []string{"ads.example.com", "tracker.example.com"}
	if !reflect.DeepEqual(res.Domains, want) {
		t.Errorf("ParseABP domains = %v, want %v", res.Domains, want)
	}
}

func TestDetectAndParse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"hosts", "0.0.0.0 a.example.com\n0.0.0.0 b.example.com\n", []string{"a.example.com", "b.example.com"}},
		{"abp", "||a.example.com^\n||b.example.com^\n", []string{"a.example.com", "b.example.com"}},
		{"plain", "a.example.com\nb.example.com\n", []string{"a.example.com", "b.example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DetectAndParse(tt.body)
			if !reflect.DeepEqual(res.Domains, tt.want) {
				t.Errorf("DetectAndParse(%s) = %v, want %v", tt.name, res.Domains, tt.want)
			}
		})
	}
}
