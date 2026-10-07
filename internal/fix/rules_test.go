package fix

import (
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/publication"
)

const testChapter2 = `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head><body><p id="sec">Two.</p></body></html>`

func testSnapshot(source string) Snapshot {
	return Snapshot{
		Workspace: WorkspaceRef{WorkspaceID: strings.Repeat("1", 32), Rootfile: "EPUB/package.opf", BaseRevision: "initial", InputTreeSHA256: strings.Repeat("0", 64)},
		Version:   "3.0",
		Container: []byte(`<container/>`),
		Package:   []byte(`<package/>`),
		Resources: []Resource{
			{Path: "EPUB/ch1.xhtml", Bytes: []byte(source)},
			{Path: "EPUB/ch2.xhtml", Bytes: []byte(testChapter2)},
		},
		Inventory: []string{"META-INF/container.xml", "EPUB/ch1.xhtml", "EPUB/ch2.xhtml", "EPUB/package.opf"},
	}
}

// TestRuleEpubTypeProhibitedElements pins FR-1 over the nine frozen elements and
// keeps an allowed position untouched.
func TestRuleEpubTypeProhibitedElements(t *testing.T) {
	source := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">` +
		`<head epub:type="a"><title epub:type="b">T</title><base epub:type="c" href="."/>` +
		`<link epub:type="d" rel="stylesheet" href="style.css"/><meta epub:type="e" name="x" content="y"/>` +
		`<noscript epub:type="f">n</noscript><script epub:type="g">var x=1;</script>` +
		`<style epub:type="h">p{}</style><template epub:type="i"><p>x</p></template></head>` +
		`<body><p epub:type="note">ok</p><aside epub:type="footnote">note</aside></body></html>`
	repairs, limits := deriveEpubType(testSnapshot(source))
	if len(repairs) != 9 {
		t.Fatalf("repairs %d: %+v", len(repairs), repairs)
	}
	if len(limits) != 0 {
		t.Fatalf("limitations: %+v", limits)
	}
	want := []string{"head", "title", "base", "link", "meta", "noscript", "script", "style", "template"}
	for i, r := range repairs {
		if r.Target.Element != want[i] || r.Status != StatusFixable || r.Operation == nil {
			t.Fatalf("repair %d: %+v", i, r)
		}
		if r.Rule.ID != RuleEpubTypeProhibited || r.Basis.Fact != FactEpubTypeProhibited {
			t.Fatalf("rule basis: %+v", r)
		}
		if len(r.ReadSet) != 3 || r.ReadSet[0] != "EPUB/ch1.xhtml" || r.ReadSet[2] != "META-INF/container.xml" {
			t.Fatalf("read set: %v", r.ReadSet)
		}
	}
	// A foreign-namespace element carrying the ops type attribute is a
	// limitation, never an expanded repair.
	foreign := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><head><title>T</title></head><body><rdf:Description epub:type="x"/></body></html>`
	repairs, limits = deriveEpubType(testSnapshot(foreign))
	if len(repairs) != 0 || len(limits) != 1 || !strings.Contains(limits[0].Reason, "foreign-namespace") {
		t.Fatalf("foreign metadata: %+v %+v", repairs, limits)
	}
}

// TestRuleRelativeURLQuery pins FR-2 scope, query removal details and the
// conservative no-repair conditions.
func TestRuleRelativeURLQuery(t *testing.T) {
	source := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body>` +
		`<a href="ch2.xhtml?q=1#sec">a</a>` +
		`<area href="ch2.xhtml?#sec"/>` +
		`<img src="ch2.xhtml?y=2"/>` +
		`<a href="ch2.xhtml#sec?not-a-query">frag</a>` +
		`<a href="ch2.xhtml%3Fq#sec">escaped</a>` +
		`<a href="ch2.xhtml#sec">plain</a>` +
		`<a href="https://example.org/x?q=1">absolute</a>` +
		`<span href="ch2.xhtml?q=1">not a URL element</span>` +
		`</body></html>`
	repairs, limits := deriveRelativeURLQuery(testSnapshot(source))
	if len(limits) != 0 {
		t.Fatalf("limitations: %+v", limits)
	}
	if len(repairs) != 3 {
		t.Fatalf("repairs %d: %+v", len(repairs), repairs)
	}
	wantValues := []string{"ch2.xhtml#sec", "ch2.xhtml#sec", "ch2.xhtml"}
	for i, r := range repairs {
		if r.Status != StatusFixable || r.Operation == nil {
			t.Fatalf("repair %d: %+v", i, r)
		}
		set, ok := r.Operation.Params.(publication.AttributeSet)
		if !ok {
			t.Fatalf("operation params: %T", r.Operation.Params)
		}
		if err := set.Validate(); err != nil {
			t.Fatalf("operation binding: %v", err)
		}
		value := set.Value
		if value != wantValues[i] {
			t.Fatalf("repair %d new value %q want %q", i, value, wantValues[i])
		}
	}
	// Query removal preserves the path spelling, an explicit empty "#" and an
	// empty query, and a missing image target stays unfixable.
	extra := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body>` +
		`<a href="ch2.xhtml?q=1#">empty-fragment</a>` +
		`<a href="ch2.xhtml?">empty-query</a>` +
		`<a href="ch2%2Exhtml?q=1#sec">percent-spelling</a>` +
		`<img src="absent.png?y=2"/>` +
		`</body></html>`
	repairs, limits = deriveRelativeURLQuery(testSnapshot(extra))
	if len(limits) != 0 || len(repairs) != 4 {
		t.Fatalf("extra repairs %d limits %+v: %+v", len(repairs), limits, repairs)
	}
	wantExtra := []struct {
		value  string
		status string
	}{
		{"ch2.xhtml#", StatusFixable},
		{"ch2.xhtml", StatusFixable},
		{"ch2%2Exhtml#sec", StatusFixable},
		{"", StatusUnfixable},
	}
	for i, w := range wantExtra {
		if repairs[i].Status != w.status {
			t.Fatalf("extra repair %d status %s want %s: %+v", i, repairs[i].Status, w.status, repairs[i])
		}
		if w.status == StatusFixable {
			set, ok := repairs[i].Operation.Params.(publication.AttributeSet)
			if !ok || set.Value != w.value {
				t.Fatalf("extra repair %d value %+v want %q", i, repairs[i].Operation.Params, w.value)
			}
		}
	}
	// An executable repair must satisfy the frozen gate exactly: a value whose
	// raw form does not resolve stays listed but unfixable.
	spaced := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><a href="  ch2.xhtml?q=1#sec  ">a</a></body></html>`
	repairs, limits = deriveRelativeURLQuery(testSnapshot(spaced))
	if len(limits) != 0 || len(repairs) != 1 || repairs[0].Status != StatusUnfixable || repairs[0].Operation != nil || repairs[0].UnfixableReason == "" {
		t.Fatalf("whitespace-padded value: %+v %+v", repairs, limits)
	}
	// A base element makes the whole document unprovable for this rule.
	based := `<html xmlns="http://www.w3.org/1999/xhtml"><head><base href="."/><title>T</title></head><body><a href="ch2.xhtml?q=1">a</a></body></html>`
	repairs, limits = deriveRelativeURLQuery(testSnapshot(based))
	if len(repairs) != 0 || len(limits) != 1 || !strings.Contains(limits[0].Reason, "base") {
		t.Fatalf("base handling: %+v %+v", repairs, limits)
	}
	// A target that only exists with the query cannot be repaired.
	missing := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><a href="absent.xhtml?q=1">a</a></body></html>`
	repairs, _ = deriveRelativeURLQuery(testSnapshot(missing))
	if len(repairs) != 1 || repairs[0].Status != StatusUnfixable || repairs[0].Operation != nil {
		t.Fatalf("missing target: %+v", repairs)
	}
	// An ambiguous fragment cannot be repaired.
	ambiguous := testSnapshot(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><a href="ch2.xhtml?q=1#sec">a</a></body></html>`)
	ambiguous.Resources[1] = Resource{Path: "EPUB/ch2.xhtml", Bytes: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head><body><p id="sec">a</p><p id="sec">b</p></body></html>`)}
	repairs, _ = deriveRelativeURLQuery(ambiguous)
	if len(repairs) != 1 || repairs[0].Status != StatusUnfixable {
		t.Fatalf("ambiguous fragment: %+v", repairs)
	}
}

// TestProposeZeroAndEpub2 pins the zero-repair proposal, the EPUB 2 limitation
// and the selection errors.
func TestProposeZeroAndEpub2(t *testing.T) {
	clean := testSnapshot(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><p>ok</p></body></html>`)
	p, err := Propose(clean, Selection{Mode: ModeAll, RepairIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Repairs) != 0 || len(p.Rules) != 0 || len(p.Derived.Operations) != 0 || p.Derived.ReadSet == nil {
		t.Fatalf("zero proposal: %+v", p)
	}
	epub2 := clean
	epub2.Version = "2.0"
	p, err = Propose(epub2, Selection{Mode: ModeAll, RepairIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Repairs) != 0 || len(p.Limitations) != 2 {
		t.Fatalf("epub2: %+v", p)
	}
	if _, err := Propose(clean, Selection{Mode: ModeExplicit, RepairIDs: []string{}}); err == nil {
		t.Fatal("empty explicit selection accepted")
	}
}
