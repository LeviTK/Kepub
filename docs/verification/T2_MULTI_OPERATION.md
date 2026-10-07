# T2 batches: multi-operation transaction foundation and XHTML structural editing

This record covers two local batches on branch `t2-multi-operation-foundation`,
which was never pushed. Batch 1 started from `origin/main`
`18618fee6b89c6ef59594c99f18f385015e0fc05` and was released by the parent at
fixed commit `23ba8f8c36b9ccf04138819acbea2af55caba832` (tree
`545f1824eb1756a979b0a554808a2bc0c0fc5c63`). Batch 2 continues from that commit
and stops at the fixed commit that introduces this section, awaiting independent
high review and parent acceptance. Private books, audit raw evidence and local
receipts are not part of Git; every fixture here is synthetic.

## Batch 1 — versioned multi-operation / multi-resource transaction foundation

Delivered: request schema 3 (`kepub-multi-v1`) with 2–256 `metadata.set` /
`content.text.set` v1 operations, frozen input binding, complete derived write
set, per-resource atomic replacement, rollback and interruption recovery,
actual-candidate review, accept settlement and history re-derivation. Not
delivered (remaining T2): structural editing, dependency synchronization,
FixProposal / ValidationDelta, batch replace and font-obfuscation eligibility.

Verification actually performed by the independent reviewer and the parent:

- Independent high review in a separate orb read every product diff and found no
  blocking defect. Third-resource hard-link failure, journal/result publish
  failure, same-OPF multiple targets, the 256/257 operation boundary and six
  baseline old-binary v1/v2 plan/active/history scenarios all passed.
- The reviewer's first ordinary full-package command hit its 600 s bound twice
  under parallel checker load; both failures were retained. The failed package
  was rerun with a 30-minute bound and passed: ordinary 242.921 s, race 427.124 s.
- The reviewer found one non-blocking evidence error: a test message and comment
  claimed a second-resource failure although every planned target shared one EPUB
  parent directory, so `chmod 0500` already failed at the first write. It was
  corrected at `23ba8f8` as a test-evidence change only, with no product change,
  and the two related tests were rerun.
- Parent probe on the final `23ba8f8` tree: all reviewer tests passed in 10.205 s,
  covering third-resource and result-publish failures, the same-OPF double field
  and wrong old value, the 256/257 boundary and the six baseline old-binary
  v1/v2 plan/active/history scenarios. The parent's earlier full-repository
  ordinary suite and vet, the transaction-targeted race tests, and the new-tree
  race/vet runs also passed.
- Batch-1 fixed inputs: commit
  `23ba8f8c36b9ccf04138819acbea2af55caba832`, tree
  `545f1824eb1756a979b0a554808a2bc0c0fc5c63`, bundle
  `t2-multi-operation-foundation.bundle` SHA-256
  `c80d573a82cfc219156842f49d1d7633971a16071eb65849a40a6fd62646f02d`. The
  parent fast-forwarded local main to `23ba8f8`; `origin/main` stayed at
  `18618fee` and nothing was pushed.

## Batch 2 — XHTML mixed content and structural editing (awaiting review)

Delivered: request schema 4 (`kepub-xhtml-structure-v1`) with 1–256 v1
operations, adding `xhtml.attribute.set`, `xhtml.attribute.remove`,
`xhtml.element.insert`, `xhtml.element.replace`, `xhtml.element.delete` and
`xhtml.element.move`. Every operation binds `bookPath`/`revisionId`/
`resourceSha256`/`locatorVersion`/exact locator and is computed as disjoint byte
edits against the frozen accepted baseline; the batch also adds the reference and
coverage gate, independent tree simulation plus block-byte verification, and
`attribute`/`element` observations in `task diff`. Not delivered (remaining T2):
cross-resource moves, FixProposal / ValidationDelta, batch replace,
font-obfuscation eligibility and CSS/resource-level dependency synchronization.

Local checks actually executed for this batch:

- `gofmt -l .` clean; `go vet ./...` exit 0.
- `go test -count=1 ./...` with the pinned EPUBCheck 5.3.0 jar: every package
  passed (cmd/kepub 255.726 s, internal/validation 214.426 s,
  internal/workspace 173.293 s, internal/publication 3.865 s,
  internal/xmltext 5.549 s, internal/references 0.099 s, others below 5 s).
- `go test -race -count=1 ./...` with the same jar: every package passed
  (cmd/kepub 420.877 s, internal/validation 231.027 s,
  internal/workspace 223.965 s, internal/publication 74.823 s,
  internal/xmltext 68.677 s, internal/references 1.554 s, others below 7 s).
- New tests in this batch: publication structure tests (edit model,
  duplicate/boundary refusal, exact splice for every operation kind including
  UTF-16, planned-path review targeting), references coverage tests (clean
  stylesheet blocks nothing, escaped CSS and scripted resources block), workspace
  structural tests (mixed content, ruby, table, footnote, direction and inline
  foreign markup fidelity; attribute/insert/replace/delete/move; stale, duplicate
  and overlapping targets; reference conflicts and coverage blockers;
  multi-resource failure and interruption recovery; review observations and
  candidate drift), and a real-binary lifecycle test (plan/apply/diff/drift/
  reject/accept/export plus the advertised capability surface).
- Real CLI smoke outside the test suite, on the final binary and a synthetic
  EPUB3 fixture: `workspace open`, `content`, `plan`, `apply`, `task diff`,
  `task accept` (pinned EPUBCheck `pass`) and `workspace export` (`verified`),
  with the exported archive re-read and checked for the exact structural edits
  and untouched resources.

Known limitations recorded by this batch, all refused explicitly rather than
approximated: one insertion per anchor position and per transaction; an insertion
point may not share an offset with a replaced range (use
`xhtml.element.replace` for substitution); fragments must be XHTML-namespace
elements without top-level text; operations apply in order, so an anchor that an
earlier operation removed is refused; cross-resource moves and CSS-level
reference maintenance remain in the remaining T2 scope.
