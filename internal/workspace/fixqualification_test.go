package workspace

import (
	"errors"
	"testing"

	"github.com/LeviTK/Kepub/internal/fix"
)

// TestFixReadOnlyQualification pins that a read-only workspace never claims an
// executable repair the legacy gates refuse, while the editable control keeps
// both repairs executable and generated markup stays explicitly unfixable.
func TestFixReadOnlyQualification(t *testing.T) {
	for _, signed := range []bool{false, true} {
		t.Run(map[bool]string{false: "editable-control", true: "signed-readonly"}[signed], func(t *testing.T) {
			extra := map[string]string{"EPUB/chapter1.xhtml": fixSource}
			if signed {
				extra["META-INF/signatures.xml"] = `<signatures xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`
			}
			w, _, _ := structureWorkspace(t, extra)
			defer w.Close()
			s, err := w.FixSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
			if err != nil {
				t.Fatal(err)
			}
			if !signed {
				if len(p.Derived.Operations) != 2 {
					t.Fatalf("editable control: %+v", p.Derived)
				}
				return
			}
			if len(w.State().ReadOnlyReasons) == 0 {
				t.Fatal("fixture is not read-only")
			}
			if len(p.Derived.Operations) != 0 || len(p.Repairs) != 2 {
				t.Fatalf("read-only proposal: operations=%d repairs=%+v", len(p.Derived.Operations), p.Repairs)
			}
			for _, r := range p.Repairs {
				if r.Status != fix.StatusUnfixable || r.Operation != nil || len(r.WriteSet) != 0 || r.UnfixableReason == "" {
					t.Fatalf("read-only repair: %+v", r)
				}
			}
			if _, err := w.Plan(editJSON(t, map[string]any{"schemaVersion": 7, "proposal": p, "operations": []fix.Operation{}})); !errors.Is(err, ErrReadOnly) {
				t.Fatalf("legacy read-only gate: %v", err)
			}
		})
	}
	for _, node := range []string{`<title epub:type="secrecy">Generated</title>`, `<a href="chapter2.xhtml?q=1#start2">Generated</a>`} {
		t.Run("generated-source", func(t *testing.T) {
			src := `<!DOCTYPE html [<!ENTITY node '` + node + `'>]><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>One</title></head><body><p id="start">One.</p>&node;</body></html>`
			w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": src})
			defer w.Close()
			s, err := w.FixSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Derived.Operations) != 0 {
				t.Fatal("generated markup acquired write authority")
			}
			if len(p.Repairs) != 1 || p.Repairs[0].Status != fix.StatusUnfixable || p.Repairs[0].Operation != nil {
				t.Fatalf("generated fact: %+v", p.Repairs)
			}
		})
	}
}
