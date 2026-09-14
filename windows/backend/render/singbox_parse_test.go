package render

import (
	"testing"

	"github.com/snowden-system/windows/backend/config"
)

// TestRenderAllChannelsPassSingboxParse — гейт «рендер → парсер ядра» (V2-046):
// каждый отрендеренный конфиг обязан проходить config.Parse (strict, с
// DisallowUnknownFields — те же типы, что sing-box). Ловит расхождение наших
// типов с типами ядра (дефект V2-045: int вместо duration-строки), не требуя сети.
func TestRenderAllChannelsPassSingboxParse(t *testing.T) {
	cc, err := RenderClientConfig(testSecrets(true))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("rendered config rejected by sing-box parser: %v", err)
	}
}
