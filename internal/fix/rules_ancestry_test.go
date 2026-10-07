package fix

import (
	"strings"
	"testing"
)

// TestRuleForeignAncestry pins the frozen scope of both native rules: an XHTML
// element that lexically re-enters the namespace below a foreign ancestor is an
// explicit limitation, never an executable repair, while the same shape under
// pure XHTML stays fixable. Coverage follows the limitations.
func TestRuleForeignAncestry(t *testing.T) {
	foreign := func(inner string) string {
		return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>T</title></head><body><p id="start">One.</p><foreign xmlns="urn:foreign"><div xmlns="http://www.w3.org/1999/xhtml">` + inner + `</div></foreign></body></html>`
	}
	direct := func(inner string) string {
		return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>T</title></head><body><p id="start">One.</p>` + inner + `</body></html>`
	}
	fr1 := `<title epub:type="secrecy">Inner</title>`
	fr2 := `<a href="ch2.xhtml?q=1#sec">Inner</a>`
	cases := []struct {
		name    string
		source  string
		foreign bool
		derive  func(Snapshot) ([]Repair, []Limitation)
	}{
		{"fr1-xhtml", direct(fr1), false, deriveEpubType},
		{"fr1-foreign", foreign(fr1), true, deriveEpubType},
		{"fr2-xhtml", direct(fr2), false, deriveRelativeURLQuery},
		{"fr2-foreign", foreign(fr2), true, deriveRelativeURLQuery},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := testSnapshot(c.source)
			repairs, limits := c.derive(s)
			if c.foreign {
				if len(repairs) != 0 {
					t.Fatalf("foreign re-entry emitted repairs: %+v", repairs)
				}
				if len(limits) != 1 || !strings.Contains(limits[0].Reason, "foreign-namespace ancestor") {
					t.Fatalf("foreign re-entry limitation: %+v", limits)
				}
				if cov := NativeCoverageOf(s); cov.Status != "partial" || len(cov.Limitations) != 1 {
					t.Fatalf("foreign coverage: %+v", cov)
				}
				if len(NativeDiagnostics(s)) != 0 {
					t.Fatal("foreign re-entry produced a native fact")
				}
				return
			}
			if len(limits) != 0 || len(repairs) != 1 || repairs[0].Status != StatusFixable {
				t.Fatalf("xhtml control: %+v %+v", repairs, limits)
			}
			if cov := NativeCoverageOf(s); cov.Status != "complete" {
				t.Fatalf("control coverage: %+v", cov)
			}
			if len(NativeDiagnostics(s)) != 1 {
				t.Fatal("control native fact missing")
			}
		})
	}
}
