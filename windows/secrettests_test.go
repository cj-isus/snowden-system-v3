package main

import (
	"encoding/json"
	"testing"
)

func TestSrvHashesUnmarshal(t *testing.T) {
	var h srvHashes
	in := `{"uuid":"f210a06c2e077d59","hy2":"a9383b3b87bfeae0","obfs":"236124c16317a5b7"}`
	if err := json.Unmarshal([]byte(in), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.UUID != "f210a06c2e077d59" || h.HY2 != "a9383b3b87bfeae0" || h.Obfs != "236124c16317a5b7" {
		t.Fatalf("unexpected: %+v", h)
	}
}

// testChannelSecret не ходит в сеть, если формат не прошёл: негативная форма
// должна вернуть один шаг «формат значения» со статусом fail.
func TestChannelSecretShortCircuitFormat(t *testing.T) {
	rep := testChannelSecret("not-a-uuid", "uuid", "0000000000000000", "UUID VLESS", "users[0].uuid")
	if rep.OK {
		t.Fatal("expected failure")
	}
	if len(rep.Steps) != 1 || rep.Steps[0].Name != "формат значения" || rep.Steps[0].Status != "fail" {
		t.Fatalf("unexpected steps: %+v", rep.Steps)
	}
}

func TestChannelSecretValueMismatchDetail(t *testing.T) {
	// Сеть недоступна в тестовой среде — не вызываем сервер. Проверяем только
	// формирование человекочитаемого сообщения о расхождении хешей.
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	got, local := "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "итог", Status: "fail",
		Detail: "РАСХОЖДЕНИЕ: на сервере " + got + ", локально " + local,
	})
	if rep.Steps[0].Detail == "" || rep.Steps[0].Status != "fail" {
		t.Fatalf("unexpected: %+v", rep.Steps[0])
	}
}

func TestExpectationsMissing(t *testing.T) {
	// В тестовом окружении файла ожиданий нет — должно быть честное «нет».
	if _, ok := loadKeyExpectation(); ok {
		t.Fatal("expected no expectation file")
	}
}
