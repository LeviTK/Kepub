package fix

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
)

const (
	goldenZeroJSON = `{"derived":{"operations":[],"readSet":[],"writeSet":[]},"limitations":[],"proposalVersion":1,"repairs":[],"rules":[],"selection":{"mode":"all","repairIds":[]},"workspace":{"baseRevision":"initial","inputTreeSha256":"0000000000000000000000000000000000000000000000000000000000000000","rootfile":"EPUB/package.opf","workspaceId":"11111111111111111111111111111111"}}`
	goldenZeroHash = "fd80e386cdf6f100ad1197bb3f9b586929850f876d4af95974b2cbe4ce54f289"

	goldenOneJSON = `{"derived":{"operations":[{"operationId":"xhtml.attribute.remove","operationVersion":1,"params":{"bookPath":"EPUB/ch1.xhtml","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"name":"type","namespace":"http://www.idpf.org/2007/ops","resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000","revisionId":"initial"}}],"readSet":["EPUB/ch1.xhtml","EPUB/package.opf","META-INF/container.xml"],"writeSet":["EPUB/ch1.xhtml"]},"limitations":[],"proposalVersion":1,"repairs":[{"basis":{"checkVersion":1,"checker":"kepub-native","fact":"xhtml-metadata-content-epub-type","spec":{"normativeLevel":"MUST NOT","section":"6.1.3.1","specVersion":"EPUB 3.3"}},"operation":{"operationId":"xhtml.attribute.remove","operationVersion":1,"params":{"bookPath":"EPUB/ch1.xhtml","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"name":"type","namespace":"http://www.idpf.org/2007/ops","resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000","revisionId":"initial"}},"readSet":["EPUB/ch1.xhtml","EPUB/package.opf","META-INF/container.xml"],"repairId":"sha256:2b44d78d9e2d1afcdc2c72204304afa97adb27d157ac9a89b1d43ef59a887976","risk":"removes an author semantic annotation that the frozen spec prohibits here","rule":{"id":"kepub.fix.epub-type-on-prohibited-element","version":1},"status":"fixable","target":{"attribute":{"name":"type","namespace":"http://www.idpf.org/2007/ops"},"bookPath":"EPUB/ch1.xhtml","element":"head","expectedOldValue":"secrecy","locator":"/html[1]/head[1]","locatorVersion":1,"resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000"},"writeSet":["EPUB/ch1.xhtml"]}],"rules":[{"id":"kepub.fix.epub-type-on-prohibited-element","version":1}],"selection":{"mode":"all","repairIds":[]},"workspace":{"baseRevision":"initial","inputTreeSha256":"0000000000000000000000000000000000000000000000000000000000000000","rootfile":"EPUB/package.opf","workspaceId":"11111111111111111111111111111111"}}`
	goldenOneHash = "40ec5378b55559eedb91c900526e6dc8dcd85fcf883758ac9976298bde11ad8a"

	goldenExplicitHash = "b8dad7dc6fdb9d9bc941eef264271b3f2f3e407864674878f6337cd47daf8874"
	goldenStringJSON   = `{"text":"\u003c\u003e\u0026é\u2028\u2029"}`
	goldenStringHash   = "d8b50ca8fdf1304c00d2b04832474db2f3e739257ecb111a31d2995badd30203"

	goldenRepairID = "sha256:2b44d78d9e2d1afcdc2c72204304afa97adb27d157ac9a89b1d43ef59a887976"
)

func digestHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func goldenWorkspace() WorkspaceRef {
	return WorkspaceRef{
		WorkspaceID:     strings.Repeat("1", 32),
		Rootfile:        "EPUB/package.opf",
		BaseRevision:    "initial",
		InputTreeSHA256: strings.Repeat("0", 64),
	}
}

// TestCanonicalGoldenVectors pins the frozen encoding vectors: the canonical
// JSON bytes and SHA-256 of the zero-repair proposal, the Unicode string
// vector, and the one-repair all and explicit proposals.
func TestCanonicalGoldenVectors(t *testing.T) {
	zero := Proposal{
		ProposalVersion: 1,
		Workspace:       goldenWorkspace(),
		Rules:           []RuleRef{},
		Selection:       Selection{Mode: ModeAll, RepairIDs: []string{}},
		Repairs:         []Repair{},
		Limitations:     []Limitation{},
		Derived:         Derived{Operations: []Operation{}, ReadSet: []string{}, WriteSet: []string{}},
	}
	b, err := zero.HashInput()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != goldenZeroJSON {
		t.Fatalf("zero canonical mismatch:\n got %s\nwant %s", b, goldenZeroJSON)
	}
	if got := digestHex(b); got != goldenZeroHash {
		t.Fatalf("zero hash %s", got)
	}
	if err := zero.Sign(); err != nil {
		t.Fatal(err)
	}
	if zero.ProposalSHA256 != goldenZeroHash || zero.ProposalID != "kepub-fix-proposal-v1:sha256:"+goldenZeroHash {
		t.Fatalf("zero signed fields: %+v", zero)
	}

	str, err := Canonical(map[string]any{"text": "<>&é\u2028\u2029"})
	if err != nil {
		t.Fatal(err)
	}
	if string(str) != goldenStringJSON {
		t.Fatalf("string vector:\n got %s\nwant %s", str, goldenStringJSON)
	}
	if got := digestHex(str); got != goldenStringHash {
		t.Fatalf("string vector hash %s", got)
	}

	one := goldenOneProposal()
	b, err = one.HashInput()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != goldenOneJSON {
		t.Fatalf("one canonical mismatch:\n got %s\nwant %s", b, goldenOneJSON)
	}
	if got := digestHex(b); got != goldenOneHash {
		t.Fatalf("one hash %s", got)
	}
	if err := one.Sign(); err != nil {
		t.Fatal(err)
	}

	explicit := one
	explicit.Selection = Selection{Mode: ModeExplicit, RepairIDs: []string{goldenRepairID}}
	explicit.DerivedFrom = &DerivedFrom{ProposalID: "kepub-fix-proposal-v1:sha256:" + goldenOneHash, ProposalSHA256: goldenOneHash}
	b, err = explicit.HashInput()
	if err != nil {
		t.Fatal(err)
	}
	if got := digestHex(b); got != goldenExplicitHash {
		t.Fatalf("explicit hash %s\ncanonical: %s", got, b)
	}
}

// goldenOneProposal builds the frozen one-repair vector document.
func goldenOneProposal() Proposal {
	op := Operation{ID: "xhtml.attribute.remove", Version: 1, Params: publication.AttributeRemove{
		BookPath:         "EPUB/ch1.xhtml",
		RevisionID:       "initial",
		ResourceSHA256:   strings.Repeat("0", 64),
		LocatorVersion:   1,
		Locator:          "/html[1]/head[1]",
		Namespace:        publication.OpsNamespace,
		Name:             "type",
		ExpectedOldValue: "secrecy",
	}}
	repair := Repair{
		RepairID: goldenRepairID,
		Rule:     ruleEpubType,
		Basis:    Basis{Checker, CheckVersion, FactEpubTypeProhibited, epubTypeSpec()},
		Target: Target{
			BookPath: "EPUB/ch1.xhtml", ResourceSHA256: strings.Repeat("0", 64),
			LocatorVersion: 1, Locator: "/html[1]/head[1]", Element: "head",
			Attribute:        AttributeRef{Namespace: publication.OpsNamespace, Name: "type"},
			ExpectedOldValue: "secrecy",
		},
		Status:    StatusFixable,
		Operation: &op,
		Risk:      RiskEpubTypeProhibited,
		ReadSet:   []string{"EPUB/ch1.xhtml", "EPUB/package.opf", "META-INF/container.xml"},
		WriteSet:  []string{"EPUB/ch1.xhtml"},
	}
	return Proposal{
		ProposalVersion: 1,
		Workspace:       goldenWorkspace(),
		Rules:           []RuleRef{ruleEpubType},
		Selection:       Selection{Mode: ModeAll, RepairIDs: []string{}},
		Repairs:         []Repair{repair},
		Limitations:     []Limitation{},
		Derived: Derived{
			Operations: []Operation{op},
			ReadSet:    []string{"EPUB/ch1.xhtml", "EPUB/package.opf", "META-INF/container.xml"},
			WriteSet:   []string{"EPUB/ch1.xhtml"},
		},
	}
}

// TestRepairIDGolden pins the deterministic repair identity derivation.
func TestRepairIDGolden(t *testing.T) {
	got := RepairID(RuleEpubTypeProhibited, 1, bookpath.BookPath("EPUB/ch1.xhtml"), "/html[1]/head[1]")
	if got != goldenRepairID {
		t.Fatalf("repair id %s", got)
	}
}
