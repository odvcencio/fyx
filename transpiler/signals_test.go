package transpiler

import (
	"strings"
	"testing"

	"github.com/odvcencio/fyrox-lang/ast"
)

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"died", "Died"},
		{"health_changed", "HealthChanged"},
		{"on_fire_started", "OnFireStarted"},
		{"a", "A"},
		{"ABC", "ABC"},
	}
	for _, tc := range tests {
		got := toPascalCase(tc.in)
		if got != tc.want {
			t.Errorf("toPascalCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSignalMsgName(t *testing.T) {
	tests := []struct {
		script, signal, want string
	}{
		{"Enemy", "died", "EnemyDiedMsg"},
		{"Player", "health_changed", "PlayerHealthChangedMsg"},
	}
	for _, tc := range tests {
		got := signalMsgName(tc.script, tc.signal)
		if got != tc.want {
			t.Errorf("signalMsgName(%q, %q) = %q, want %q", tc.script, tc.signal, got, tc.want)
		}
	}
}

func TestTranspileSignalStructs(t *testing.T) {
	signals := []ast.Signal{
		{
			Name: "died",
			Params: []ast.Param{
				{Name: "position", TypeExpr: "Vector3"},
			},
		},
	}
	out := TranspileSignalStructs("Enemy", signals)
	if !strings.Contains(out, "#[derive(Debug, Clone)]") {
		t.Errorf("missing derive: %s", out)
	}
	if !strings.Contains(out, "pub struct EnemyDiedMsg {") {
		t.Errorf("missing struct EnemyDiedMsg: %s", out)
	}
	if !strings.Contains(out, "pub position: Vector3,") {
		t.Errorf("missing field: %s", out)
	}
}

func TestTranspileSignalStructsMultiple(t *testing.T) {
	signals := []ast.Signal{
		{
			Name:   "died",
			Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}},
		},
		{
			Name: "damaged",
			Params: []ast.Param{
				{Name: "amount", TypeExpr: "f32"},
				{Name: "source", TypeExpr: "Handle<Node>"},
			},
		},
	}
	out := TranspileSignalStructs("Enemy", signals)
	if !strings.Contains(out, "struct EnemyDiedMsg") {
		t.Errorf("missing EnemyDiedMsg: %s", out)
	}
	if !strings.Contains(out, "struct EnemyDamagedMsg") {
		t.Errorf("missing EnemyDamagedMsg: %s", out)
	}
	if !strings.Contains(out, "pub amount: f32,") {
		t.Errorf("missing amount field: %s", out)
	}
	if !strings.Contains(out, "pub source: Handle<Node>,") {
		t.Errorf("missing source field: %s", out)
	}
}

func TestTranspileSignalStructsEmpty(t *testing.T) {
	out := TranspileSignalStructs("Enemy", nil)
	if out != "" {
		t.Errorf("expected empty output for no signals, got: %q", out)
	}
}

func TestTranspileSignalStructsNoParams(t *testing.T) {
	signals := []ast.Signal{
		{Name: "reset", Params: nil},
	}
	out := TranspileSignalStructs("Game", signals)
	if !strings.Contains(out, "pub struct GameResetMsg {") {
		t.Errorf("missing struct: %s", out)
	}
	if !strings.Contains(out, "}") {
		t.Errorf("missing closing brace: %s", out)
	}
}

func TestTranspileConnectSubscriptions(t *testing.T) {
	connects := []ast.Connect{
		{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 1;"},
	}
	out := TranspileConnectSubscriptions(connects)
	if !strings.Contains(out, "ctx.message_dispatcher.subscribe_to::<EnemyDiedMsg>(ctx.handle);") {
		t.Errorf("missing subscribe_to: %s", out)
	}
}

func TestTranspileConnectSubscriptionsMultiple(t *testing.T) {
	connects := []ast.Connect{
		{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 1;"},
		{ScriptName: "Player", SignalName: "health_changed", Params: []string{"hp"}, Body: "update_ui(hp);"},
	}
	out := TranspileConnectSubscriptions(connects)
	if !strings.Contains(out, "subscribe_to::<EnemyDiedMsg>") {
		t.Errorf("missing EnemyDiedMsg subscribe: %s", out)
	}
	if !strings.Contains(out, "subscribe_to::<PlayerHealthChangedMsg>") {
		t.Errorf("missing PlayerHealthChangedMsg subscribe: %s", out)
	}
}

func TestTranspileConnectSubscriptionsEmpty(t *testing.T) {
	out := TranspileConnectSubscriptions(nil)
	if out != "" {
		t.Errorf("expected empty output for no connects, got: %q", out)
	}
}

func TestTranspileConnectDispatch(t *testing.T) {
	connects := []ast.Connect{
		{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 100;"},
	}
	out := TranspileConnectDispatch(connects)
	if !strings.Contains(out, "if let Some(msg) = message.downcast_ref::<EnemyDiedMsg>()") {
		t.Errorf("missing if-let downcast: %s", out)
	}
	if !strings.Contains(out, "let pos = &msg.pos;") {
		t.Errorf("missing param binding: %s", out)
	}
	if !strings.Contains(out, "self.score += 100;") {
		t.Errorf("missing body: %s", out)
	}
}

func TestTranspileConnectDispatchMultiple(t *testing.T) {
	connects := []ast.Connect{
		{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 100;"},
		{ScriptName: "Player", SignalName: "health_changed", Params: []string{"hp"}, Body: "update_ui(hp);"},
	}
	out := TranspileConnectDispatch(connects)
	if !strings.Contains(out, "downcast_ref::<EnemyDiedMsg>()") {
		t.Errorf("missing EnemyDiedMsg dispatch: %s", out)
	}
	if !strings.Contains(out, "downcast_ref::<PlayerHealthChangedMsg>()") {
		t.Errorf("missing PlayerHealthChangedMsg dispatch: %s", out)
	}
	if !strings.Contains(out, "self.score += 100;") {
		t.Errorf("missing first body: %s", out)
	}
	if !strings.Contains(out, "update_ui(hp);") {
		t.Errorf("missing second body: %s", out)
	}
}

func TestTranspileConnectDispatchEmpty(t *testing.T) {
	out := TranspileConnectDispatch(nil)
	if out != "" {
		t.Errorf("expected empty output for no connects, got: %q", out)
	}
}

func TestTranspileConnectDispatchMultipleParams(t *testing.T) {
	connects := []ast.Connect{
		{
			ScriptName: "Enemy",
			SignalName: "damaged",
			Params:     []string{"amount", "source"},
			Body:       "log::info!(\"hit for {}\", amount);",
		},
	}
	out := TranspileConnectDispatch(connects)
	if !strings.Contains(out, "let amount = &msg.amount;") {
		t.Errorf("missing amount binding: %s", out)
	}
	if !strings.Contains(out, "let source = &msg.source;") {
		t.Errorf("missing source binding: %s", out)
	}
}

func TestRewriteEmitGlobal(t *testing.T) {
	signals := []ast.Signal{
		{Name: "died", Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}}},
	}
	body := "emit died(self.position());"
	out := RewriteEmitStatements(body, "Enemy", signals)
	if !strings.Contains(out, "ctx.message_sender.send_global(EnemyDiedMsg { position: self.position() });") {
		t.Errorf("emit not rewritten correctly: %s", out)
	}
}

func TestRewriteEmitTargeted(t *testing.T) {
	signals := []ast.Signal{
		{
			Name: "damaged",
			Params: []ast.Param{
				{Name: "amount", TypeExpr: "f32"},
				{Name: "source", TypeExpr: "Handle<Node>"},
			},
		},
	}
	body := "emit damaged(10.0, ctx.handle) to target;"
	out := RewriteEmitStatements(body, "Enemy", signals)
	if !strings.Contains(out, "ctx.message_sender.send_to_target(target, EnemyDamagedMsg { amount: 10.0, source: ctx.handle });") {
		t.Errorf("targeted emit not rewritten correctly: %s", out)
	}
}

func TestRewriteEmitNoSignals(t *testing.T) {
	body := "self.speed += 1.0;"
	out := RewriteEmitStatements(body, "Enemy", nil)
	if out != body {
		t.Errorf("body without emit should be unchanged, got: %s", out)
	}
}

func TestRewriteEmitMultiple(t *testing.T) {
	signals := []ast.Signal{
		{Name: "died", Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}}},
		{Name: "damaged", Params: []ast.Param{{Name: "amount", TypeExpr: "f32"}}},
	}
	body := "emit died(pos);\nemit damaged(10.0);"
	out := RewriteEmitStatements(body, "Enemy", signals)
	if !strings.Contains(out, "send_global(EnemyDiedMsg { position: pos })") {
		t.Errorf("first emit not rewritten: %s", out)
	}
	if !strings.Contains(out, "send_global(EnemyDamagedMsg { amount: 10.0 })") {
		t.Errorf("second emit not rewritten: %s", out)
	}
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"a, b", []string{"a", "b"}},
		{"foo(a, b), c", []string{"foo(a, b)", "c"}},
		{"x", []string{"x"}},
		{"", nil},
		{"a, b, c", []string{"a", "b", "c"}},
	}
	for _, tc := range tests {
		got := splitArgs(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitArgs(%q) got %d items, want %d: %v", tc.in, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitArgs(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

// TestTranspileSignals is the integration test from the task spec.
func TestTranspileSignals(t *testing.T) {
	scripts := []ast.Script{
		{
			Name: "Enemy",
			Signals: []ast.Signal{
				{Name: "died", Params: []ast.Param{{Name: "position", TypeExpr: "Vector3"}}},
			},
		},
		{
			Name: "Tracker",
			Connects: []ast.Connect{
				{ScriptName: "Enemy", SignalName: "died", Params: []string{"pos"}, Body: "self.score += 1;"},
			},
		},
	}
	out := TranspileSignalStructs("Enemy", scripts[0].Signals)
	if !strings.Contains(out, "struct EnemyDiedMsg") {
		t.Errorf("missing message struct: %s", out)
	}

	subs := TranspileConnectSubscriptions(scripts[1].Connects)
	if !strings.Contains(subs, "subscribe_to::<EnemyDiedMsg>") {
		t.Errorf("missing subscribe_to: %s", subs)
	}
}
