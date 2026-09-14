package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/config"
)

// TestSplitAbsentByDefault — без split_direct в наборе ни правила, ни
// find_process в конфиге быть не должно (fail-open guard).
func TestSplitAbsentByDefault(t *testing.T) {
	channels := mustChannels(t)
	src := testSecrets(true)
	splitSet = nil
	defer func() { splitSet = nil }()
	cc, err := RenderFrom(channels, src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(cc.Raw), "process_name") || strings.Contains(string(cc.Raw), "find_process") {
		t.Fatalf("empty split must not render process rules:\n%s", cc.Raw)
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("sing-box parser: %v", err)
	}
}

// TestSplitRenderedFromSet — список из набора рендерится правилом ПЕРЕД
// RFC1918 и включает find_process; конфиг проходит строгий парсер ядра.
// ВАЖНО: splitSet присваивается ПОСЛЕ mustChannels (LoadDescriptors сбрасывает
// сплит из встроенного набора — проверяем именно рендер-этап).
func TestSplitRenderedFromSet(t *testing.T) {
	channels := mustChannels(t)
	src := testSecrets(true)
	splitSet = []string{"steam.exe", "backup-helper.exe"}
	defer func() { splitSet = nil }()
	cc, err := RenderFrom(channels, src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var raw struct {
		Route struct {
			Rules []struct {
				ProcessName []string `json:"process_name"`
				Outbound    string   `json:"outbound"`
			} `json:"rules"`
			FindProcess bool `json:"find_process"`
		} `json:"route"`
	}
	if err := json.Unmarshal(cc.Raw, &raw); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !raw.Route.FindProcess {
		t.Fatalf("find_process must be enabled with non-empty split")
	}
	found := false
	for _, r := range raw.Route.Rules {
		if len(r.ProcessName) > 0 {
			found = true
			if r.Outbound != "direct" {
				t.Fatalf("split rule must be direct, got %q", r.Outbound)
			}
			if r.ProcessName[0] != "steam.exe" || r.ProcessName[1] != "backup-helper.exe" {
				t.Fatalf("split names lost: %v", r.ProcessName)
			}
			break
		}
	}
	if !found {
		t.Fatalf("process rule missing:\n%s", cc.Raw)
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("sing-box parser rejected split config: %v", err)
	}
}

// TestValidateSplitDirectNormalizes — нормализация регистра/пробелов,
// дубликаты схлопываются, пути отбрасываются.
func TestValidateSplitDirectNormalizes(t *testing.T) {
	got := validateSplitDirect([]string{"Steam.EXE", " steam.exe ", `C:\ evil\app.exe`, "", "other.exe"})
	if len(got) != 2 || got[0] != "steam.exe" || got[1] != "other.exe" {
		t.Fatalf("validateSplitDirect = %v", got)
	}
}
