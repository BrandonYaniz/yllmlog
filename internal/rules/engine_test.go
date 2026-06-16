package rules

import "testing"

func TestEngineMatchers(t *testing.T) {
	rules := []Rule{
		{ID: 1, Name: "contains", Source: SourceSystemDefault, Matcher: MatcherContains, Pattern: "timeout", Action: ActionAnalyze, Priority: 100, Enabled: true},
		{ID: 2, Name: "prefix", Source: SourceSystemDefault, Matcher: MatcherPrefix, Pattern: "fatal:", Action: ActionEscalate, Priority: 100, Enabled: true},
		{ID: 3, Name: "suffix", Source: SourceSystemDefault, Matcher: MatcherSuffix, Pattern: "ignored", Action: ActionIgnore, Priority: 100, Enabled: true},
		{ID: 4, Name: "regex", Source: SourceSystemDefault, Matcher: MatcherRegex, Pattern: `pid=\d+`, Action: ActionGroup, Priority: 100, Enabled: true},
		{ID: 5, Name: "field", Source: SourceSystemDefault, Matcher: MatcherFieldEquals, Field: "facility", Pattern: "auth", Action: ActionReport, Priority: 100, Enabled: true},
	}
	engine, err := NewEngine(rules)
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}

	tests := []struct {
		name   string
		ctx    LineContext
		action Action
	}{
		{name: "contains", ctx: LineContext{Line: "request timeout"}, action: ActionAnalyze},
		{name: "prefix", ctx: LineContext{Line: "fatal: disk missing"}, action: ActionEscalate},
		{name: "suffix", ctx: LineContext{Line: "noise ignored"}, action: ActionIgnore},
		{name: "regex", ctx: LineContext{Line: "worker pid=123"}, action: ActionGroup},
		{name: "field", ctx: LineContext{Line: "login failed", Fields: map[string]string{"facility": "auth"}}, action: ActionReport},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, ok := engine.Match(tt.ctx)
			if !ok {
				t.Fatal("Match returned no result")
			}
			if match.Action != tt.action {
				t.Fatalf("Action = %q, want %q", match.Action, tt.action)
			}
		})
	}
}

func TestEngineOrdersBySourceThenPriority(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{ID: 1, Name: "llm", Source: SourceLLMGenerated, Matcher: MatcherContains, Pattern: "panic", Action: ActionAnalyze, Priority: 1, Enabled: true},
		{ID: 2, Name: "admin", Source: SourceAdminCreated, Matcher: MatcherContains, Pattern: "panic", Action: ActionEscalate, Priority: 100, Enabled: true},
		{ID: 3, Name: "override", Source: SourceAdminOverride, Matcher: MatcherContains, Pattern: "panic", Action: ActionReport, Priority: 50, Enabled: true},
	})
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}

	match, ok := engine.Match(LineContext{Line: "kernel panic"})
	if !ok {
		t.Fatal("Match returned no result")
	}
	if match.Rule.ID != 3 {
		t.Fatalf("Rule ID = %d, want 3", match.Rule.ID)
	}
}

func TestEngineRejectsInvalidRegex(t *testing.T) {
	_, err := NewEngine([]Rule{
		{ID: 1, Name: "bad", Source: SourceSystemDefault, Matcher: MatcherRegex, Pattern: `[`, Action: ActionAnalyze, Priority: 100, Enabled: true},
	})
	if err == nil {
		t.Fatal("NewEngine accepted invalid regex")
	}
}
