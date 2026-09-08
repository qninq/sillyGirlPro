package core

import (
	"testing"

	"github.com/qninq/sillyGirlPro/core/common"
)

func TestFmtRuleCompilesRulesOnceAndKeepsParamsAligned(t *testing.T) {
	fn := &common.Function{
		Rules: []string{"hello [name]", "raw ^ping$"},
	}
	fmtRule(fn)

	if len(fn.RulePatterns) != len(fn.Rules) {
		t.Fatalf("compiled rules length = %d; want %d", len(fn.RulePatterns), len(fn.Rules))
	}
	if len(fn.Params) != len(fn.Rules) {
		t.Fatalf("params length = %d; want %d", len(fn.Params), len(fn.Rules))
	}
	if fn.RulePatterns[0] == nil || fn.RulePatterns[1] == nil {
		t.Fatalf("expected compiled rules: %#v", fn.RulePatterns)
	}
	if got := fn.RulePatterns[0].FindStringSubmatch("hello world"); len(got) != 2 || got[1] != "world" {
		t.Fatalf("compiled rule did not match param: %#v", got)
	}
	if len(fn.Params[0]) != 1 || fn.Params[0][0] != "name" {
		t.Fatalf("params = %#v; want name", fn.Params)
	}

	fmtRule(fn)
	if len(fn.Params) != len(fn.Rules) {
		t.Fatalf("second fmtRule params length = %d; want %d", len(fn.Params), len(fn.Rules))
	}
}
