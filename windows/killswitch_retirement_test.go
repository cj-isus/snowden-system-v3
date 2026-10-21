package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// Static boundary guard complements the injected policy tests: a state-file PID
// must never become authority to terminate a process, and no new lease may start.
func TestRetiredHelperHasNoProcessTerminationOrLeaseStart(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "killswitch_windows.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		switch fn.Name.Name {
		case "startKillSwitchHelper", "renewKillSwitchLease", "applyKillSwitch":
			t.Errorf("retired lease entrypoint remains: %s", fn.Name.Name)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if ok && lit.Kind == token.STRING {
			value, _ := strconv.Unquote(lit.Value)
			if value == "taskkill" {
				t.Error("a marker PID can still reach taskkill")
			}
		}
		return true
	})
}

func TestRetiredHelperFlagsCannotBecomeGUILaunches(t *testing.T) {
	for _, args := range [][]string{
		{"--killswitch-expire", "--killswitch-seconds=90"},
		{"--killswitch-expire"}, {"--killswitch-seconds"},
		{"--killswitch-seconds", "90"}, {"--killswitch-seconds=90"},
		{"--killswitch-expire=invalid"}, {"--killswitch-seconds="},
		{"--killswitch-expire", "--killswitch-seconds=0"},
		{"--killswitch-expire", "--killswitch-seconds=-1"},
		{"--killswitch-expire", "--killswitch-seconds=7200"},
		{"--killswitch-expire", "--killswitch-seconds=garbage"},
		{"--tun", "--killswitch-expire", "--auto-connect"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if !legacyKillSwitchHelperRequested(args) {
				t.Fatal("retired helper arguments would fall through to GUI")
			}
		})
	}
	for _, args := range [][]string{nil, {"--tun"}, {"--auto-connect"}, {"--killswitch-expire-other"}, {"--killswitch-seconds-other=90"}} {
		t.Run("GUI/"+strings.Join(args, " "), func(t *testing.T) {
			if legacyKillSwitchHelperRequested(args) {
				t.Fatal("unrelated GUI arguments incorrectly retired")
			}
		})
	}
}

// Execute no Wails/UI code in an offline test. Check the production dispatch
// boundary structurally; the flag classifier is exercised behaviorally above.
func TestRetiredHelperGuardIsBeforeEveryMainSideEffect(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" {
			continue
		}
		guard, ok := fn.Body.List[0].(*ast.IfStmt)
		if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 1 {
			t.Fatal("first main action must be the side-effect-free helper guard")
		}
		call, ok := guard.Cond.(*ast.CallExpr)
		if !ok {
			t.Fatal("helper guard is not the classifier")
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "legacyKillSwitchHelperRequested" {
			t.Fatal("helper classifier is not first")
		}
		if _, ok := guard.Body.List[0].(*ast.ReturnStmt); !ok {
			t.Fatal("retired helper must return without timer/firewall/update effects")
		}
		return
	}
	t.Fatal("main entrypoint missing")
}
