package fix

import (
	"reflect"
	"testing"
)

func upstreamSide(status string, ruleset string, diags ...UpstreamDiagnostic) DeltaSide {
	return DeltaSide{
		Checks:   []CheckMeta{{CheckerID: "epubcheck", ToolVersion: "5.3.0", Ruleset: ruleset, RunStatus: status, Coverage: "full packaged EPUB conformance"}},
		Upstream: diags,
	}
}

func upstreamDiag(code, severity, path string, line int, message string) UpstreamDiagnostic {
	return UpstreamDiagnostic{Code: code, Severity: severity, BookPath: path, Line: line, Column: 1, Message: message}
}

func nativeSide(status string, diags ...NativeDiagnostic) DeltaSide {
	return DeltaSide{
		Checks:            []CheckMeta{{CheckerID: Checker, ToolVersion: "1", Ruleset: "kepub-fix-native-v1", RunStatus: "completed", Coverage: NativeCoverage{Scope: NativeScope, Status: status}}},
		NativeDiagnostics: diags,
	}
}

func nativeFact(path string) NativeDiagnostic {
	return NativeDiagnostic{Source: Checker, CheckVersion: CheckVersion, Rule: RuleRef{ID: RuleEpubTypeProhibited, Version: 1}, Fact: "epub:type-on-prohibited-element", Severity: "error", Target: Target{BookPath: path, Locator: "l1", Attribute: AttributeRef{Namespace: "http://www.idpf.org/2007/ops", Name: "type"}}}
}

func single(t *testing.T, entries []DeltaEntry, source string) DeltaEntry {
	t.Helper()
	var found []DeltaEntry
	for _, e := range entries {
		if e.Source == source {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one %s entry, got %+v", source, entries)
	}
	return found[0]
}

// TestClassifyDeltaUpstreamVocabulary pins the frozen classification of the
// upstream diagnostics: generic codes, severity changes and duplicates.
func TestClassifyDeltaUpstreamVocabulary(t *testing.T) {
	site := upstreamDiag("RSC-033", "error", "EPUB/chapter1.xhtml", 2, "query component")
	before := upstreamSide("completed", "3.3", site)
	after := upstreamSide("completed", "3.3")
	if e := single(t, ClassifyDelta(before, after), "epubcheck"); e.Classification != "resolved" {
		t.Fatalf("specific code: %+v", e)
	}
	if e := single(t, ClassifyDelta(upstreamSide("completed", "3.3", upstreamDiag("RSC-005", "error", "EPUB/chapter1.xhtml", 2, "parse error")), after), "epubcheck"); e.Classification != "incomparable" {
		t.Fatalf("generic code: %+v", e)
	}
	if e := single(t, ClassifyDelta(before, upstreamSide("completed", "3.3", site)), "epubcheck"); e.Classification != "persisted" || len(e.Before) != 1 || len(e.After) != 1 {
		t.Fatalf("persisted: %+v", e)
	}
	if e := single(t, ClassifyDelta(after, before), "epubcheck"); e.Classification != "added" {
		t.Fatalf("added: %+v", e)
	}
	// A severity change on the same site is one upgraded entry, not a removal
	// plus an addition.
	lower := upstreamSide("completed", "3.3", upstreamDiag("RSC-033", "warning", "EPUB/chapter1.xhtml", 2, "query component"))
	if e := single(t, ClassifyDelta(lower, before), "epubcheck"); e.Classification != "upgraded" || len(e.Before) != 1 || len(e.After) != 1 {
		t.Fatalf("upgraded: %+v", e)
	}
	if e := single(t, ClassifyDelta(before, lower), "epubcheck"); e.Classification != "incomparable" || len(e.After) != 1 {
		t.Fatalf("severity decrease: %+v", e)
	}
	// Duplicate instances are preserved and never offset.
	dup := upstreamSide("completed", "3.3", site, site)
	if e := single(t, ClassifyDelta(dup, after), "epubcheck"); e.Classification != "resolved" || len(e.Before) != 2 || len(e.After) != 0 {
		t.Fatalf("duplicates: %+v", e)
	}
}

// TestClassifyDeltaInstanceAndComparability pins the frozen classifier rules
// that the medium counterexamples exercised: instance pairing, real tool and
// configuration identity, coverage direction and generic attribution.
func TestClassifyDeltaInstanceAndComparability(t *testing.T) {
	site := upstreamDiag("RSC-033", "error", "EPUB/chapter1.xhtml", 8, "query component")
	// A duplicate decrease is one persisted pair plus one resolved instance.
	entries := ClassifyDelta(upstreamSide("completed", "3.3", site, site), upstreamSide("completed", "3.3", site))
	persisted, resolved := 0, 0
	for _, e := range entries {
		if e.Classification == "persisted" {
			if len(e.Before) != len(e.After) {
				t.Fatalf("persisted multiplicities: %+v", e)
			}
			persisted += len(e.After)
		}
		if e.Classification == "resolved" {
			resolved += len(e.Before)
		}
	}
	if persisted != 1 || resolved != 1 {
		t.Fatalf("instance split: %+v", entries)
	}
	// Actual tool bytes and configuration are comparability evidence.
	for _, mutate := range []func(*DeltaSide, *DeltaSide){
		func(b, a *DeltaSide) { b.Checks[0].ToolSHA256, a.Checks[0].ToolSHA256 = "old-tool", "new-tool" },
		func(b, a *DeltaSide) { b.Checks[0].ConfigHash, a.Checks[0].ConfigHash = "strict-false", "strict-true" },
		func(b, a *DeltaSide) { a.Checks[0].Coverage = "only EPUB/chapter2.xhtml" },
	} {
		b, a := upstreamSide("completed", "3.3", site), upstreamSide("completed", "3.3")
		mutate(&b, &a)
		if e := single(t, ClassifyDelta(b, a), "epubcheck"); e.Classification != "incomparable" {
			t.Fatalf("comparability: %+v", e)
		}
	}
	// A generic schema code has no provable site, so it is never upgraded.
	genericBefore := upstreamSide("completed", "3.3", upstreamDiag("RSC-005", "warning", "EPUB/chapter1.xhtml", 8, "schema mismatch"))
	genericAfter := upstreamSide("completed", "3.3", upstreamDiag("RSC-005", "error", "EPUB/chapter1.xhtml", 8, "schema mismatch"))
	if e := single(t, ClassifyDelta(genericBefore, genericAfter), "epubcheck"); e.Classification != "incomparable" {
		t.Fatalf("generic attribution: %+v", e)
	}
}

// TestClassifyDeltaCoverageDirection pins the reliable-coverage direction: a
// diagnostic entering a reliable range is newly_checkable, and a checker that
// did not run reliably never claims a resolution.
func TestClassifyDeltaCoverageDirection(t *testing.T) {
	site := upstreamDiag("RSC-033", "error", "EPUB/chapter1.xhtml", 2, "query component")
	if e := single(t, ClassifyDelta(upstreamSide("unavailable", "3.3"), upstreamSide("completed", "3.3", site)), "epubcheck"); e.Classification != "newly_checkable" {
		t.Fatalf("newly checkable: %+v", e)
	}
	if e := single(t, ClassifyDelta(upstreamSide("completed", "3.3", site), upstreamSide("unavailable", "3.3")), "epubcheck"); e.Classification != "incomparable" {
		t.Fatalf("unreliable after side: %+v", e)
	}
	if e := single(t, ClassifyDelta(upstreamSide("completed", "3.3", site), upstreamSide("completed", "other-ruleset")), "epubcheck"); e.Classification != "incomparable" {
		t.Fatalf("configuration mismatch: %+v", e)
	}
}

// TestClassifyDeltaNativeCoverage pins the native scope status direction.
func TestClassifyDeltaNativeCoverage(t *testing.T) {
	fact := nativeFact("EPUB/chapter1.xhtml")
	if e := single(t, ClassifyDelta(nativeSide("complete", fact), nativeSide("complete")), Checker); e.Classification != "resolved" {
		t.Fatalf("native resolved: %+v", e)
	}
	if e := single(t, ClassifyDelta(nativeSide("partial", fact), nativeSide("complete")), Checker); e.Classification != "incomparable" {
		t.Fatalf("native partial before: %+v", e)
	}
	if e := single(t, ClassifyDelta(nativeSide("partial"), nativeSide("complete", fact)), Checker); e.Classification != "newly_checkable" {
		t.Fatalf("native newly checkable: %+v", e)
	}
}

// Persist equal severities first, pair only the remaining instances, and keep
// every surplus. Three severities also exercise consumption across groups.
func TestClassifyDeltaSeverityMultiplicity(t *testing.T) {
	w := upstreamDiag("RSC-033", "warning", "EPUB/chapter1.xhtml", 8, "query component")
	e := w
	e.Severity = "error"
	f := w
	f.Severity = "fatal"
	for _, tc := range []struct {
		name          string
		before, after []UpstreamDiagnostic
		want          map[string][2]int
	}{
		{"before-surplus", []UpstreamDiagnostic{w, w}, []UpstreamDiagnostic{e}, map[string][2]int{"upgraded": {1, 1}, "resolved": {1, 0}}},
		{"after-surplus", []UpstreamDiagnostic{w}, []UpstreamDiagnostic{e, e}, map[string][2]int{"upgraded": {1, 1}, "added": {0, 1}}},
		{"persisted-before-surplus", []UpstreamDiagnostic{w, w, e}, []UpstreamDiagnostic{e, e}, map[string][2]int{"persisted": {1, 1}, "upgraded": {1, 1}, "resolved": {1, 0}}},
		{"persisted-after-surplus", []UpstreamDiagnostic{w, e}, []UpstreamDiagnostic{e, e, e}, map[string][2]int{"persisted": {1, 1}, "upgraded": {1, 1}, "added": {0, 1}}},
		{"multiple-addition-groups", []UpstreamDiagnostic{w, w}, []UpstreamDiagnostic{e, f}, map[string][2]int{"upgraded": {2, 2}}},
		{"multiple-removal-groups", []UpstreamDiagnostic{w, e}, []UpstreamDiagnostic{f, f}, map[string][2]int{"upgraded": {2, 2}}},
		{"single-control", []UpstreamDiagnostic{w}, []UpstreamDiagnostic{e}, map[string][2]int{"upgraded": {1, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := ClassifyDelta(upstreamSide("completed", "3.3", tc.before...), upstreamSide("completed", "3.3", tc.after...))
			got := map[string][2]int{}
			for _, entry := range entries {
				if entry.Classification == "persisted" || entry.Classification == "upgraded" {
					if len(entry.Before) != len(entry.After) {
						t.Fatalf("unequal pair: %+v", entry)
					}
				}
				n := got[entry.Classification]
				got[entry.Classification] = [2]int{n[0] + len(entry.Before), n[1] + len(entry.After)}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("instances: got %v want %v; %+v", got, tc.want, entries)
			}
		})
	}
}

func TestClassifyDeltaNativeMultiplicity(t *testing.T) {
	first := nativeFact("EPUB/chapter1.xhtml")
	first.Target.ExpectedOldValue = "first"
	second := first
	second.Target.ExpectedOldValue = "second"
	for _, tc := range []struct {
		name   string
		before DeltaSide
		after  DeltaSide
		class  string
	}{
		{"decrease", nativeSide("complete", first, second), nativeSide("complete", first), "resolved"},
		{"increase", nativeSide("complete", first), nativeSide("complete", first, second), "added"},
		{"partial-decrease", nativeSide("partial", first, second), nativeSide("complete", first), "incomparable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := ClassifyDelta(tc.before, tc.after)
			if len(entries) != 2 || entries[0].Classification != "persisted" || entries[1].Classification != tc.class {
				t.Fatalf("split: %+v", entries)
			}
			if !reflect.DeepEqual(entries[0].Before, []any{first}) || !reflect.DeepEqual(entries[0].After, []any{first}) {
				t.Fatalf("persisted instance: %+v", entries[0])
			}
			b, a := []any{second}, []any{}
			if tc.class == "added" {
				b, a = a, b
			}
			if !reflect.DeepEqual(entries[1].Before, b) || !reflect.DeepEqual(entries[1].After, a) {
				t.Fatalf("surplus instance (not reused persisted): %+v", entries[1])
			}
		})
	}
}
