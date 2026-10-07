package fix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// NativeDiagnostic is one native rule fact of a delta side. It never claims an
// upstream checker code.
type NativeDiagnostic struct {
	Source       string  `json:"source"`
	CheckVersion int     `json:"checkVersion"`
	Rule         RuleRef `json:"rule"`
	Fact         string  `json:"fact"`
	Severity     string  `json:"severity"`
	Target       Target  `json:"target"`
}

// NativeCoverage is the coverage statement of the native check.
type NativeCoverage struct {
	Scope       string       `json:"scope"`
	Status      string       `json:"status"`
	Limitations []Limitation `json:"limitations"`
}

// NativeScope is the frozen native coverage scope identifier.
const NativeScope = "manifest-xhtml-native-fix-v1"

// NativeDiagnostics derives the native facts of one frozen snapshot.
func NativeDiagnostics(s Snapshot) []NativeDiagnostic {
	f := derive(s)
	out := []NativeDiagnostic{}
	for _, r := range f.repairs {
		out = append(out, NativeDiagnostic{Checker, CheckVersion, r.Rule, r.Basis.Fact, "error", r.Target})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Rule.ID != b.Rule.ID {
			return a.Rule.ID < b.Rule.ID
		}
		if a.Target.BookPath != b.Target.BookPath {
			return a.Target.BookPath < b.Target.BookPath
		}
		return a.Target.Locator < b.Target.Locator
	})
	return out
}

// NativeCoverageOf reports the native scope status of one frozen snapshot.
func NativeCoverageOf(s Snapshot) NativeCoverage {
	f := derive(s)
	status := "complete"
	switch {
	case !s.epub3():
		status = "not_applicable"
	case len(f.limitations) > 0:
		status = "partial"
	}
	return NativeCoverage{Scope: NativeScope, Status: status, Limitations: f.limitations}
}

// NativeReportHash binds the native coverage and diagnostics without reusing the
// validation report hash.
func NativeReportHash(c NativeCoverage, d []NativeDiagnostic) string {
	b, err := Canonical(map[string]any{"coverage": c, "diagnostics": d})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CheckMeta is one checker metadata entry of a delta side.
type CheckMeta struct {
	CheckerID    string `json:"checkerId"`
	ToolVersion  string `json:"toolVersion"`
	ToolSHA256   string `json:"toolSha256"`
	Ruleset      string `json:"ruleset"`
	SpecBaseline string `json:"specBaseline"`
	Profile      string `json:"profile"`
	Flags        string `json:"flags"`
	InputHash    string `json:"inputHash"`
	ConfigHash   string `json:"configHash"`
	ReportHash   string `json:"reportHash"`
	RunStatus    string `json:"runStatus"`
	Coverage     any    `json:"coverage"`
}

// DeltaRef binds one side snapshot to its real revision or task identity.
type DeltaRef struct {
	WorkspaceID     string `json:"workspaceId"`
	Rootfile        string `json:"rootfile"`
	BaseRevision    string `json:"baseRevision"`
	InputTreeSHA256 string `json:"inputTreeSha256"`
	RevisionID      string `json:"revisionId,omitempty"`
	TaskID          string `json:"taskId,omitempty"`
	ExecutionSHA256 string `json:"executionSha256,omitempty"`
	TreeSHA256      string `json:"treeSha256"`
}

// UpstreamDiagnostic preserves one checker diagnostic exactly as reported.
type UpstreamDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	BookPath string `json:"bookPath"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
}

// DeltaSide is one side of a validation delta.
type DeltaSide struct {
	Snapshot          DeltaRef             `json:"snapshot"`
	Checks            []CheckMeta          `json:"checks"`
	Report            any                  `json:"report"`
	ReportHash        string               `json:"reportHash"`
	NativeDiagnostics []NativeDiagnostic   `json:"nativeDiagnostics"`
	Upstream          []UpstreamDiagnostic `json:"upstreamDiagnostics"`
}

// DeltaEntry is one classified diagnostic difference.
type DeltaEntry struct {
	Classification string `json:"classification"`
	Source         string `json:"source"`
	Identity       any    `json:"identity"`
	Before         []any  `json:"before"`
	After          []any  `json:"after"`
	Reason         string `json:"reason"`
}

// Delta is the frozen ValidationDelta v1 document.
type Delta struct {
	DeltaVersion int          `json:"deltaVersion"`
	Before       DeltaSide    `json:"before"`
	After        DeltaSide    `json:"after"`
	Entries      []DeltaEntry `json:"entries"`
	Limitations  []Limitation `json:"limitations"`
}

// DeltaVersion is the frozen delta wire version.
const DeltaVersion = 1

// ReportHash binds one complete validation report exactly as marshaled.
func ReportHash(report any) string {
	b, err := json.Marshal(report)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// nativeKey is the provable native identity: rule, fact, resource, locator and
// attribute. The expected old value never creates another identity.
type nativeKey struct {
	Rule      string
	Fact      string
	BookPath  string
	Locator   string
	Namespace string
	Name      string
}

func nativeIdentity(d NativeDiagnostic) nativeKey {
	return nativeKey{d.Rule.ID, d.Fact, d.Target.BookPath, d.Target.Locator, d.Target.Attribute.Namespace, d.Target.Attribute.Name}
}

// upstreamKey is the exact checker identity; generic schema errors are never
// matched by code alone.
type upstreamKey struct {
	Code, Severity, BookPath string
	Line, Column             int
	Message                  string
}

func upstreamIdentity(d UpstreamDiagnostic) upstreamKey {
	return upstreamKey{d.Code, d.Severity, d.BookPath, d.Line, d.Column, d.Message}
}

// genericSchemaCode reports whether one upstream code cannot be attributed to a
// single rule instance, so its disappearance must not be called resolved.
func genericSchemaCode(code string) bool {
	switch code {
	case "RSC-005", "RSC-016", "RSC-017":
		return true
	}
	return false
}

// checksComparable reports whether the two sides ran the same checker
// configuration over a comparable scope. Checks are matched by checker
// identity, never cross pairs; coverage direction is classified separately.
func checksComparable(before, after DeltaSide) bool {
	if len(before.Checks) == 0 || len(after.Checks) == 0 {
		return false
	}
	for _, b := range before.Checks {
		matched := false
		for _, a := range after.Checks {
			if b.CheckerID != a.CheckerID {
				continue
			}
			matched = true
			if b.ToolVersion != a.ToolVersion || b.ToolSHA256 != a.ToolSHA256 || b.ConfigHash != a.ConfigHash ||
				b.Ruleset != a.Ruleset || b.SpecBaseline != a.SpecBaseline ||
				b.Profile != a.Profile || b.Flags != a.Flags || b.RunStatus != a.RunStatus {
				return false
			}
		}
		if !matched {
			return false
		}
	}
	for _, a := range after.Checks {
		matched := false
		for _, b := range before.Checks {
			if a.CheckerID == b.CheckerID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// ClassifyDelta compares two sides by diagnostic multiset. Counts never offset,
// duplicates are preserved, and unprovable identities stay incomparable.
func ClassifyDelta(before, after DeltaSide) []DeltaEntry {
	entries := []DeltaEntry{}
	comparable := checksComparable(before, after)
	beforeNative := map[nativeKey][]NativeDiagnostic{}
	afterNative := map[nativeKey][]NativeDiagnostic{}
	for _, d := range before.NativeDiagnostics {
		beforeNative[nativeIdentity(d)] = append(beforeNative[nativeIdentity(d)], d)
	}
	for _, d := range after.NativeDiagnostics {
		afterNative[nativeIdentity(d)] = append(afterNative[nativeIdentity(d)], d)
	}
	keys := map[nativeKey]bool{}
	for k := range beforeNative {
		keys[k] = true
	}
	for k := range afterNative {
		keys[k] = true
	}
	nativeKeys := make([]nativeKey, 0, len(keys))
	for k := range keys {
		nativeKeys = append(nativeKeys, k)
	}
	sort.Slice(nativeKeys, func(i, j int) bool {
		a, b := nativeKeys[i], nativeKeys[j]
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.BookPath != b.BookPath {
			return a.BookPath < b.BookPath
		}
		return a.Locator < b.Locator
	})
	beforeStatus, afterStatus := "", ""
	if len(before.Checks) > 0 {
		if c, ok := before.Checks[len(before.Checks)-1].Coverage.(NativeCoverage); ok {
			beforeStatus = c.Status
		}
	}
	if len(after.Checks) > 0 {
		if c, ok := after.Checks[len(after.Checks)-1].Coverage.(NativeCoverage); ok {
			afterStatus = c.Status
		}
	}
	for _, k := range nativeKeys {
		b, a := beforeNative[k], afterNative[k]
		switch {
		case len(b) > 0 && len(a) > 0:
			entries = append(entries, DeltaEntry{"persisted", Checker, k, nativeInstances(b), nativeInstances(a), "the same native rule fact is present on both sides"})
		case len(b) > 0 && len(a) == 0:
			reason := "the native fact is gone and the native scope still covers it"
			classification := "resolved"
			if !comparable || beforeStatus != "complete" || afterStatus != "complete" {
				classification, reason = "incomparable", "the native scope or checker configuration is not comparable"
			}
			entries = append(entries, DeltaEntry{classification, Checker, k, nativeInstances(b), []any{}, reason})
		case len(b) == 0 && len(a) > 0:
			classification, reason := "added", "a new native fact appears in the after snapshot"
			switch {
			case !comparable || afterStatus != "complete":
				classification, reason = "incomparable", "the native scope or checker configuration is not comparable"
			case beforeStatus != "complete":
				classification, reason = "newly_checkable", "the after native scope reliably covers a fact the before scope did not"
			}
			entries = append(entries, DeltaEntry{classification, Checker, k, []any{}, nativeInstances(a), reason})
		}
	}
	entries = append(entries, classifyUpstream(before, after, comparable)...)
	return entries
}

func nativeInstances(in []NativeDiagnostic) []any {
	out := make([]any, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}

func classifyUpstream(before, after DeltaSide, comparable bool) []DeltaEntry {
	entries := []DeltaEntry{}
	beforeUp := map[upstreamKey][]UpstreamDiagnostic{}
	afterUp := map[upstreamKey][]UpstreamDiagnostic{}
	for _, d := range before.Upstream {
		beforeUp[upstreamIdentity(d)] = append(beforeUp[upstreamIdentity(d)], d)
	}
	for _, d := range after.Upstream {
		afterUp[upstreamIdentity(d)] = append(afterUp[upstreamIdentity(d)], d)
	}
	keys := map[upstreamKey]bool{}
	for k := range beforeUp {
		keys[k] = true
	}
	for k := range afterUp {
		keys[k] = true
	}
	upstreamKeys := make([]upstreamKey, 0, len(keys))
	for k := range keys {
		upstreamKeys = append(upstreamKeys, k)
	}
	sort.Slice(upstreamKeys, func(i, j int) bool {
		a, b := upstreamKeys[i], upstreamKeys[j]
		if a.BookPath != b.BookPath {
			return a.BookPath < b.BookPath
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Code < b.Code
	})
	beforeReliable, afterReliable := upstreamReliable(before), upstreamReliable(after)
	comparable = comparable && beforeReliable && afterReliable &&
		upstreamCoverage(before) == upstreamCoverage(after)
	type surplus struct {
		key   upstreamKey
		extra int
	}
	removals, additions := []surplus{}, []surplus{}
	for _, k := range upstreamKeys {
		b, a := beforeUp[k], afterUp[k]
		n := min(len(b), len(a))
		if n > 0 {
			entries = append(entries, DeltaEntry{"persisted", "epubcheck", k, upstreamInstances(b[:n]), upstreamInstances(a[:n]), "the same checker diagnostic is present on both sides"})
		}
		if len(b) > n {
			removals = append(removals, surplus{k, len(b) - n})
		}
		if len(a) > n {
			additions = append(additions, surplus{k, len(a) - n})
		}
	}
	// A severity change on the same diagnostic site is one entry, never a
	// removal plus an addition; a generic schema code has no provable site.
	usedRemoval := make([]bool, len(removals))
	usedAddition := make([]bool, len(additions))
	for i, r := range removals {
		for j, a := range additions {
			if usedAddition[j] || upstreamSite(r.key) != upstreamSite(a.key) {
				continue
			}
			usedRemoval[i], usedAddition[j] = true, true
			b, av := beforeUp[r.key], afterUp[a.key]
			classification, reason := "incomparable", "the checker severity changed without a provable resolution"
			if severityRank(av[0].Severity) > severityRank(b[0].Severity) && !genericSchemaCode(r.key.Code) {
				classification, reason = "upgraded", "the checker severity increased on the same diagnostic site"
			}
			if !comparable {
				classification, reason = "incomparable", "the checker configuration or coverage is not comparable"
			}
			entries = append(entries, DeltaEntry{classification, "epubcheck", r.key, upstreamInstances(b[:r.extra]), upstreamInstances(av[:a.extra]), reason})
			break
		}
	}
	for i, r := range removals {
		if usedRemoval[i] {
			continue
		}
		b := beforeUp[r.key]
		classification, reason := "resolved", "the checker diagnostic is gone and the checker still covers the resource"
		if !comparable || genericSchemaCode(r.key.Code) {
			classification, reason = "incomparable", "a generic schema diagnostic has no provable unique attribution"
		}
		entries = append(entries, DeltaEntry{classification, "epubcheck", r.key, upstreamInstances(b[:r.extra]), []any{}, reason})
	}
	for j, a := range additions {
		if usedAddition[j] {
			continue
		}
		av := afterUp[a.key]
		classification, reason := "added", "a new checker diagnostic appears in the after snapshot"
		switch {
		case !afterReliable:
			classification, reason = "incomparable", "the after checker did not reliably cover the resource"
		case !beforeReliable:
			classification, reason = "newly_checkable", "the after checker reliably covers a diagnostic the before side did not check"
		case !comparable || genericSchemaCode(a.key.Code):
			classification, reason = "incomparable", "a generic schema diagnostic has no provable unique attribution"
		}
		entries = append(entries, DeltaEntry{classification, "epubcheck", a.key, []any{}, upstreamInstances(av[:a.extra]), reason})
	}
	return entries
}

// upstreamReliable reports whether one side ran the checker to a reliable
// result. A normal compliance FAIL is still a reliable run.
func upstreamReliable(s DeltaSide) bool {
	for _, c := range s.Checks {
		if c.CheckerID == "epubcheck" {
			return c.RunStatus == "completed"
		}
	}
	return false
}

// upstreamSite is the diagnostic identity without its severity.
func upstreamSite(k upstreamKey) upstreamKey {
	k.Severity = ""
	return k
}

// severityRank orders the checker severities for the upgrade classification.
func severityRank(s string) int {
	switch s {
	case "fatal":
		return 4
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	}
	return 0
}

// upstreamCoverage is the covered scope of the upstream checker on one side.
func upstreamCoverage(s DeltaSide) string {
	for _, c := range s.Checks {
		if c.CheckerID == "epubcheck" {
			return fmt.Sprint(c.Coverage)
		}
	}
	return ""
}

func upstreamInstances(in []UpstreamDiagnostic) []any {
	out := make([]any, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}
