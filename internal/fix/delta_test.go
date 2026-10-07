package fix

import "testing"

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
