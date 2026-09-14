package main

// adaptive_test.go — unit-тесты классификатора адаптивного слоя (V2-041).
// Live-проверка через реальный туннель идёт отдельно (не в этом файле):
// классификация страниц-челленджей — чистая функция, гонять сеть не нужно.

import (
	"testing"
)

func TestIsChallengeBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"cloudflare js-challenge", "Just a moment... <script>enable JavaScript and cookies to continue</script>", true},
		{"cf-chl marker", "window._cf_chl_opt={...}", true},
		{"attention required", "<title>Attention Required! | Cloudflare</title>", true},
		{"openai unusual activity", "unusual activity has been detected", true},
		{"plain 403 without marker", "403 Forbidden: access denied by policy", false},
		{"empty body", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isChallengeBody([]byte(tc.body)); got != tc.want {
				t.Fatalf("isChallengeBody(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
