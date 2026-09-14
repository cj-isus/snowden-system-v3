package main

// Живой тест COM-пути netguard.go из не-elevated процесса:
//   go test . -run TestLiveNetGuard -v
// Задачи NetGuard на машине владельца установлены, поэтому здесь ожидаются
// честные факты: user-задача = ready с датами, SYSTEM-задача = no-access
// (скрыта правами от не-elevated процесса — это факт, не ошибка).

import (
	"encoding/json"
	"testing"
)

func TestLiveNetGuardStatus(t *testing.T) {
	app := NewApp()
	st, err := app.NetGuardStatus()
	if err != nil {
		t.Fatalf("NetGuardStatus: %v", err)
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	t.Logf("NetGuardStatus:\n%s", b)

	if len(st.Tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(st.Tasks))
	}
	for _, task := range st.Tasks {
		switch task.Status {
		case "ready", "running", "disabled", "queued", "no-access", "not-found":
			// честные факты
		default:
			if len(task.Status) > 8 && task.Status[:8] == "unknown:" {
				t.Errorf("задача %s: нераспознанный HRESULT: %s", task.Name, task.Status)
			}
		}
		t.Logf("task %s: status=%s last=%q next=%q code=%q", task.Name, task.Status, task.LastRun, task.NextRun, task.LastCode)
	}
}
