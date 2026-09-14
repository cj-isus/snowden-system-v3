package render

import (
	"os"
	"testing"

	"github.com/snowden-system/windows/backend/metadata"
)

// TestMain — изоляция тестов от реального AppData-хранилища машины.
// LoadDescriptors читает %AppData%\snowden-system\metadata (FR-008); если на
// машине разработчика развёрнут настоящий envelope, юнит-тесты «встроенного
// набора» видели бы подменённые данные (поймано live 2026-09-11: тест
// TestLoadDescriptorsValid увидел 1 канал вместо 2 после установки тестового
// envelope в AppData). Все тесты рендера идут с пустой временной сторой;
// live-поведение с настоящей сторой — отдельные тесты с явной подменой
// (envelope_override_test.go) и live-проверки владельца.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "render-test-metadata")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	old := envelopeStore
	envelopeStore = &metadata.Store{Dir: dir}
	code := m.Run()
	envelopeStore = old
	os.Exit(code)
}
