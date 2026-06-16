package rules

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type MatcherType string
type Action string
type Source string

const (
	MatcherContains       MatcherType = "contains"
	MatcherPrefix         MatcherType = "prefix"
	MatcherSuffix         MatcherType = "suffix"
	MatcherRegex          MatcherType = "regex"
	MatcherFieldEquals    MatcherType = "field_equals"
	MatcherFieldContains  MatcherType = "field_contains"
	MatcherServicePattern MatcherType = "service_pattern"

	ActionIgnore   Action = "ignore"
	ActionAnalyze  Action = "analyze"
	ActionGroup    Action = "group"
	ActionEscalate Action = "escalate"
	ActionReport   Action = "report"

	SourceSystemDefault Source = "system_default"
	SourceLLMGenerated  Source = "llm_generated"
	SourceAdminCreated  Source = "admin_created"
	SourceAdminOverride Source = "admin_override"
	SourceLLMModified   Source = "llm_modified"
)

type Rule struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Source   Source      `json:"source"`
	Matcher  MatcherType `json:"matcher"`
	Pattern  string      `json:"pattern"`
	Field    string      `json:"field,omitempty"`
	Action   Action      `json:"action"`
	Priority int         `json:"priority"`
	Enabled  bool        `json:"enabled"`
}

type LineContext struct {
	Line    string
	Service string
	Fields  map[string]string
}

type Match struct {
	Rule   Rule
	Action Action
}

type Engine struct {
	rules []compiledRule
}

type compiledRule struct {
	rule  Rule
	regex *regexp.Regexp
}

func NewEngine(rules []Rule) (Engine, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		compiledRule, err := compile(rule)
		if err != nil {
			return Engine{}, err
		}
		compiled = append(compiled, compiledRule)
	}
	sort.SliceStable(compiled, func(i, j int) bool {
		left := compiled[i].rule
		right := compiled[j].rule
		if sourceRank(left.Source) != sourceRank(right.Source) {
			return sourceRank(left.Source) < sourceRank(right.Source)
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return left.ID < right.ID
	})
	return Engine{rules: compiled}, nil
}

func (e Engine) Match(ctx LineContext) (Match, bool) {
	for _, rule := range e.rules {
		if rule.matches(ctx) {
			return Match{Rule: rule.rule, Action: rule.rule.Action}, true
		}
	}
	return Match{}, false
}

func compile(rule Rule) (compiledRule, error) {
	if strings.TrimSpace(rule.Name) == "" {
		return compiledRule{}, errors.New("rule name is required")
	}
	if strings.TrimSpace(rule.Pattern) == "" {
		return compiledRule{}, fmt.Errorf("rule %q pattern is required", rule.Name)
	}
	if rule.Action == "" {
		return compiledRule{}, fmt.Errorf("rule %q action is required", rule.Name)
	}

	compiled := compiledRule{rule: rule}
	switch rule.Matcher {
	case MatcherContains, MatcherPrefix, MatcherSuffix, MatcherServicePattern:
		return compiled, nil
	case MatcherFieldEquals, MatcherFieldContains:
		if strings.TrimSpace(rule.Field) == "" {
			return compiledRule{}, fmt.Errorf("rule %q field is required", rule.Name)
		}
		return compiled, nil
	case MatcherRegex:
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return compiledRule{}, fmt.Errorf("compile rule %q regex: %w", rule.Name, err)
		}
		compiled.regex = regex
		return compiled, nil
	default:
		return compiledRule{}, fmt.Errorf("rule %q uses unsupported matcher %q", rule.Name, rule.Matcher)
	}
}

func (r compiledRule) matches(ctx LineContext) bool {
	switch r.rule.Matcher {
	case MatcherContains:
		return strings.Contains(ctx.Line, r.rule.Pattern)
	case MatcherPrefix:
		return strings.HasPrefix(ctx.Line, r.rule.Pattern)
	case MatcherSuffix:
		return strings.HasSuffix(ctx.Line, r.rule.Pattern)
	case MatcherRegex:
		return r.regex.MatchString(ctx.Line)
	case MatcherFieldEquals:
		return ctx.Fields[r.rule.Field] == r.rule.Pattern
	case MatcherFieldContains:
		return strings.Contains(ctx.Fields[r.rule.Field], r.rule.Pattern)
	case MatcherServicePattern:
		matched, err := filepathMatch(r.rule.Pattern, ctx.Service)
		return err == nil && matched
	default:
		return false
	}
}

func filepathMatch(pattern, value string) (bool, error) {
	if strings.ContainsAny(pattern, "*?[") {
		return filepath.Match(pattern, value)
	}
	return pattern == value, nil
}

func sourceRank(source Source) int {
	switch source {
	case SourceAdminOverride:
		return 0
	case SourceAdminCreated:
		return 1
	case SourceLLMModified:
		return 2
	case SourceSystemDefault:
		return 3
	case SourceLLMGenerated:
		return 4
	default:
		return 5
	}
}
