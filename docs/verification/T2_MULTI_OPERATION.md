# T2 batches: multi-operation transaction foundation and XHTML structural editing

This record covers two local batches on branch `t2-multi-operation-foundation`,
which was never pushed. Batch 1 started from `origin/main`
`18618fee6b89c6ef59594c99f18f385015e0fc05` and was released by the parent at
fixed commit `23ba8f8c36b9ccf04138819acbea2af55caba832` (tree
`545f1824eb1756a979b0a554808a2bc0c0fc5c63`). Batch 2 continues from that commit
and stops at the fixed commit that introduces this section, awaiting independent
medium review and parent acceptance; batch 2 was accepted at `79a4548` and
released by the parent, and batch 3 (native literal/regex batch text
replacement) is in progress. Batch 2's first four trees were rejected —
`ff64188` and `f295f3f` by the high reviewer, `6f9980e` by the medium reviewer,
`b301fb9` by the parent's acceptance verification and the medium reviewer — and
every result stays bound to the reviewer and tree that produced it. Private
books, audit raw evidence and local receipts are not part of Git; every fixture
here is synthetic.

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
- The reviewer's first full-package commands hit the 600 s bound under parallel
  checker load, once for the ordinary suite and once for the race suite. Both
  failures were retained, and the failed packages were rerun with a 30-minute
  bound and passed: ordinary 242.921 s, race 427.124 s.
- The reviewer found one non-blocking evidence error: a test message and comment
  claimed a second-resource failure although every planned target shared one EPUB
  parent directory, so `chmod 0500` already failed at the first write. It was
  corrected at `23ba8f8` as a test-evidence change only, with no product change,
  and the two related tests were rerun.
- Baseline and increment were measured separately: the complete repository
  ordinary and race suites plus vet were executed on the pre-change tree at
  `18618fee` before the batch-1 changes landed, and after the changes the complete
  suites were rerun on the new tree together with the batch's new multi-operation
  tests.
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

## Batch 2 — XHTML mixed content and structural editing (first four trees rejected; accepted and released at `79a4548`)

The first batch-2 fixed tree, commit `ff64188b4d073aa665fa7e352910c75468d0e218`
(tree `1d76e4e59088aa3b7e47275ec5587980e20791ce`, bundle
`t2-multi-operation-foundation.bundle` SHA-256
`ecfcae58d2fa9b1cdc76d9f9c6a86a072d494f0e118047754dc0ae3b3d55a7be`), was
**formally rejected** by the independent high review and reproduced by the
parent. The frozen probes and their reproduction logs are retained in the review
thread under `.amp/in/artifacts/t2b-review/`; the received copies in this
workspace use a `.go.received` suffix and are not committed.

### Rejection evidence (ten blocker groups)

1. `TestParentStructureNewLinkToRemovedID` (parent) and
   `TestReviewerStructureFinalReferenceGate` (reviewer): the dependency gate
   validated per-resource partial state, so a new link to an identity the same
   transaction removes was accepted when the link's resource was processed first
   and correctly refused in the other order. The same order dependence applied to
   re-pointing an existing `href` inside one resource.
2. `TestReviewerStructureOldTextManifestRestriction`: schema 4 with a legal
   attribute operation let `content.text.set` v1 write an unregistered resource
   that schema 2 refuses, widening the older permission boundary.
3. `TestReviewerStructureIDREFCoverage`: `headers` IDREF references were not
   extracted, so removing a table-header identity named by `td headers` was
   allowed and left a dangling reference. (The fix also extracts the other HTML
   and ARIA IDREF attributes from the same source-code contract; only `headers`
   was empirically reproduced, and no record here claims the rest were tested.)
4. `TestReviewerStructureTextReviewFollowsShift`: `task diff` read an old
   `content.text.set` target by its frozen locator after another operation
   inserted a preceding sibling, reporting a different element's text while
   `matchesExecution` stayed true.
5. `TestReviewerStructureVerifierRejectsMisplacedBlock`: block placement checks
   only ran for nodes that carried a frozen locator, so a `last-child` insertion
   placed before trailing text was accepted by the independent verifier.
6. `TestReviewerStructureAttributePositiveCases`: three legitimate writes failed
   — an empty old attribute value had no writable interval, an apostrophe in a
   single-quoted value produced invalid XML, and a first operations-namespace
   declaration was written without a separating space.
7. `TestReviewerStructureUnsupportedURLWrite`: an `iframe srcdoc` fragment
   bypassed the script and URL gate.
8. `TestReviewerStructureMoveReview`: a successful move was reported as
   `block bytes missing (occurrences 1→1)`.
9. `TestReviewerStructureFragmentIDValidation`: a fragment `id="bad id"` was
   inserted despite the NCName contract.
10. `TestReviewerStructureSchemaMoveAnchor`: the advertised
    `xhtml.element.move` anchor schema reused the attribute-name pattern and
    rejected a valid structural locator.

Transmission hygiene also failed once: probe files copied to the repository root
with a `.go` suffix were scanned as a root package, so the reviewer's race and
vet runs exited 1 on a compile error. Those exits are retained and do not count
as passes; the defect was transport, not product.

Suite results for this rejected tree (`ff64188`, tree `1d76e4e`), each bound to
the runner that produced it:

| runner | ordinary (cmd/kepub, workspace, validation) | race (cmd/kepub, workspace, validation) |
|---|---|---|
| batch author | 255.726 s, 173.293 s, 214.426 s (publication 3.865 s, xmltext 5.549 s, references 0.099 s) | 420.877 s, 223.965 s, 231.027 s (publication 74.823 s, xmltext 68.677 s, references 1.554 s) |
| high reviewer (clean reruns) | 309.699 s, 216.502 s, 259.478 s | 500.342 s, 266.523 s, 276.432 s |

The reviewer's vet and `git diff --check` were clean; a passing suite does not
offset the ten probe failures.

### Fixes after the first rejection (minimal, one rule per cause)

- The dependency gate validates the whole transaction's **final** identity and
  link state in three phases (derive, collect, validate), so operation order
  cannot change acceptance or the write set.
- Structural edit targets must be manifest `application/xhtml+xml` items, reusing
  the single-operation permission check.
- IDREF attributes are references with a per-syntax coverage entry.
- `task diff` locates text and attribute targets by their planned tree path.
- Every inserted, replaced and moved block is verified at its final position:
  block nodes get synthetic locators, `first-child`/`last-child` require
  adjacency to the parent's tags, `before`/`after` require adjacency to the
  anchor, replacements require their frozen character-data context (with a
  documented fallback when another operation inserts into that context; both the
  context rule and the fallback were superseded by the positional rule in the
  second-rejection fix below), and a count guard refuses a silent skip.
- Attribute writes escape both quote characters, accept empty value intervals,
  and emit namespace declarations with correct separators and a reused or free
  prefix.
- Fragments refuse dynamic content and unsupported URL attributes and require
  legal XML names for identities.
- `xhtml.element.move` advertises a locator for its anchor.

### Second rejection: high review of `f295f3f` and reviewer handover

The corrected tree `f295f3f` (tree `d194ff34`) was itself rejected by the
independent high review after the parent reproduced its findings. Two blocker
groups remained, and that tree's own suite results — the batch author's ordinary
`cmd/kepub` 307.456 s, `internal/workspace` 213.876 s, `internal/validation`
260.150 s (publication 4.674 s, xmltext 6.465 s, references 0.139 s) and race
`cmd/kepub` 499.009 s, `internal/workspace` 271.587 s, `internal/validation`
274.137 s (publication 88.984 s, xmltext 80.023 s, references 1.861 s), plus the
high reviewer's ordinary `cmd/kepub` 310.894 s, `internal/workspace` 217.916 s,
`internal/validation` 264.098 s and race `cmd/kepub` 507.167 s,
`internal/workspace` 275.164 s, `internal/validation` 279.907 s, with vet and
`git diff --check` clean — do not offset them:

1. **New IDREF facts were not gated.** `xhtml.attribute.set` and fragments could
   write `headers` or ARIA IDREF values that point at an identity which is
   missing or removed in the same transaction, in either operation order
   (`TestParentStructureNewIDREFToRemovedID`, `TestReviewerR2NewIDREF`). The fix
   shares one IDREF vocabulary between the reference index and the structural
   edit facts, collects IDREF tokens from new attribute values and fragments, and
   validates every token against the transaction's final identity state of that
   resource. ARIA IDREF behaviour is now empirically reproduced, not inferred.
2. **Replacement blocks were not positionally verified.** The gap-insertion
   fallback and the whole-output `Contains` check accepted a replacement placed
   after trailing text, including when another context elsewhere in the document
   carried the same surrounding text (`TestReviewerR2ReplacePlacement`). The fix
   requires every block at the offset the frozen edit facts independently imply
   (accumulated size changes of earlier edits), with no `Contains` search and no
   degraded branch; point insertions additionally keep their parent/anchor
   adjacency checks.

The reviewer role then changed: the original high reviewer stopped taking new
reviews after handing over its evidence and finishing the in-flight race run, and
an independent reviewer on the user's configured medium mode took over. Records
keep the two roles separate; earlier high results are not relabelled.

### Third rejection: medium review of `6f9980e`

The fixed tree `6f9980e` (tree `d84d803a`) was **formally rejected** by the
independent medium review, and the parent reproduced the finding. That tree's own
clean results — the batch author's working-tree ordinary suite (cmd/kepub
301.855 s, internal/validation 252.483 s, internal/workspace 209.884 s,
internal/publication 4.598 s, internal/xmltext 6.598 s, internal/references
0.138 s) and race suite (cmd/kepub 499.055 s, internal/validation 277.039 s,
internal/workspace 274.639 s, internal/publication 86.383 s, internal/xmltext
79.395 s, internal/references 1.843 s), the medium reviewer's clean ordinary
suite (cmd/kepub 368.990 s, internal/workspace 262.188 s, internal/validation
311.937 s) and race suite (cmd/kepub 604.318 s, internal/workspace 347.226 s,
internal/validation 353.708 s) with vet exit 0, its real CLI run (18.472 s, no
skips, correct error codes and refusals without plans, fixed-checker
accept/export, 6 ZIP byte checks, original book preserved) and its fuzz runs
(structural round trip 1,055,117 executions in 61.012 s, plan order independence
30,865 executions in 61.056 s, independent byte-oracle block placement 186,668
executions in 60.095 s) — do not offset the blocker:

1. **Final identity counts were wrong.** The gate tracked identity removal as a
   boolean, so deleting one of two same-value instances erased both baseline
   counts and then added one, hiding a residual duplicate: deleting the first of
   two `id="unreferenced"` elements and inserting a new one let `aria-labelledby`
   plan and apply while the candidate still had two target elements
   (`TestMediumIDREFFinalIdentityMultiplicity`). The stronger extension kept an
   `xml:id` alias while removing `id` and still accepted the new duplicate in
   both operation orders, and the positive control — deleting one of two
   same-value instances and referencing the survivor — was wrongly refused
   (`TestMediumIDREFRetainedAlias`, `TestMediumIDREFPartialDeletePositive`).
   `id` and `xml:id` must count per target element, deduplicated per element, and
   the IDREF/href gates must share that same true final state.
2. **Record binding errors (non-product).** The previous record attributed the
   high reviewer's `ff64188` suite times to `f295f3f`, moved that tree's own
   numbers into this revision, and stated fuzz numbers for `6f9980e` that were
   taken on an earlier tree's working tree. The record is corrected in the next
   revision: every number is bound to the tree and runner that produced it, and
   working-tree runs are labelled as working-tree runs. Fuzz runs stay with their
   trees: physical interval invariants 943,012 executions in 61 s on `ff64188`
   (`internal/xmltext` unchanged since), structural round trip 1,464,358
   executions in 61 s and plan order independence 48,047 executions in 61 s on
   the `f295f3f` working tree. The parent's own partial verification of
   `6f9980e` (frozen probes and UTF positive controls, including their race runs,
   passed without skips; a live structural round-trip fuzz passed 299,071
   executions in 20 s) is bound to that rejected tree as well.

The frozen probes for the rejected trees (`parent_structure_probe_test.go`,
`reviewer_structure_probe_test.go`, `reviewer_structure_schema_test.go`,
`parent_final_idref_probe_test.go`, `reviewer_r2_probe_test.go`,
`reviewer_medium_idref_test.go` and the extended
`reviewer_medium_idref_extended_test.go`) and the UTF positive control
(`reviewer_r2_positive_test.go`, 12 encoding subcases) were received with the
stated SHA-256 values and are kept as `.go.received` files that are not
committed.

### Fixes in this revision (per-element identity unit, minimal)

1. **One identity unit shared by the reference index and the gate.** New
   `publication.IdentityValues`/`ElementIdentities`/`CountIDs` count identity
   per element: an element carrying both `id` and `xml:id` with one value is one
   identity, and distinct elements with the same value are distinct identities.
   The reference index, the structural gate and the review evidence all derive
   from this unit, so `id`/`xml:id` aliasing and per-element deduplication cannot
   disagree between them.
2. **Counts, not flags, for the final state.** Subtree facts and (at this tree)
   attribute facts computed their identity deltas from the element's identity
   set; the gate keeps integer `removed`/`added` maps and derives `finalCount =
   base - removed + added` per resource and value. A new identity is refused
   while any residual frozen instance remains, a removed identity is only checked
   against frozen edges when it truly reaches zero, and a re-added value keeps
   its count so a legal partial delete plus reference stays accepted. The
   per-attribute part of this rule was itself rejected and is superseded below.
3. **Redundant replacement search removed.** The `replace` placement check still
   runs the precise mapped-offset byte equality and the structural parent/order
   comparison, and no longer keeps the whole-output `bytes.Contains` fallback, so
   the contract has one verification rule rather than a weaker second one.

### Fourth rejection: joint attribute identity facts on `b301fb9`

The parent's acceptance verification of `b301fb9` (tree `9cb9ad3a`) reproduced a
joint-edit blocker with a 3055-byte probe (`parent_b301_alias_probe_test.go`,
SHA-256 `ed1e6889499565f37108ee6b2e6ae4ed7b80566cd204ed56686e8267c839520f`): 16
subcases (four edit groups × two operation orders × `aria-labelledby`/`href`)
failed in ordinary and focused race runs, while the older frozen sets passed. The
medium reviewer independently confirmed the same 16 failures and did not release
the tree. That tree's own runs, taken from its working tree before the commit, do
not offset the blocker: the repository ordinary suite passed all packages
(cmd/kepub 306.781 s, internal/validation 258.668 s, internal/workspace 215.969 s,
internal/publication 4.877 s, internal/xmltext 6.402 s, internal/references
0.127 s) and the race suite passed all packages (cmd/kepub 505.364 s,
internal/validation 281.626 s, internal/workspace 281.831 s,
internal/publication 92.144 s, internal/xmltext 78.036 s, internal/references
1.791 s), with `go vet` and `gofmt` clean, the structural fuzzes passing (828,274
and 44,193 executions in 61 s), the complete frozen probe set then passing (19
functions, 118 assertions) and the CLI smoke covering the earlier identity cases.

The blocker: `AttributeSetEdit`/`AttributeRemoveEdit` derived identity deltas
from the frozen element one attribute at a time and the gate summed them, but two
attribute changes on one node cannot be represented by per-attribute deltas:
removing `id` and `xml:id` with the same value, or renaming both to distinct new
values, still let a reference to the removed value plan and apply while the
candidate had no such identity; renaming both to the same new value was refused
as a duplicate although the final node carries one identity; and removing `id`
while rewriting `xml:id` to the removed value was refused as missing although the
final node carries exactly that identity.

The medium reviewer's adjacent evidence pack (21373 B, SHA-256
`928d2cbc02b87bf360073201a66e1fd27ee3e9ecb4c904e6730ff9485095bc09`) extended the
same cause: an existing frozen ARIA/href reference was left dangling after a
joint removal (four cases), a legal swap of two identity values between `id` and
`xml:id` was refused as missing (two cases), and the effective-transaction fuzz
target found `remove id + xml:id → freshY` still allowing `href="#oldX"`; the
minimal seed `21d0b33faf043b04` replayed with exit 1. A non-blocking diagnostic
regression also appeared in the shared-unit conversion: the reference index
consumed identity values by value alone, so `title="note"` before `id="note"`
moved the `DUPLICATE_ID` location to `/@title` while the reversed order was
correct.

### Fixes after the fourth rejection (node-level identity merge, minimal)

1. **One merged identity fact per frozen node.** New
   `publication.MergedIdentityDelta` applies all of a node's frozen attribute
   changes together to the frozen attribute list and returns the identity values
   that node loses and gains; attribute edits no longer report per-attribute
   identity deltas. Old-value checks, the new-id NCName rule and byte-interval
   restrictions are unchanged.
2. **The gate collects attribute facts per node.** Phase 2 groups the
   transaction's attribute edits by locator, derives one merged delta per node and
   adds it to the resource's counts, so the final identity state does not depend
   on how many attributes of one node were touched. Element edits keep their
   subtree facts.
3. **Permanent regressions.** `TestStructureJointAliasEdits` covers the four
   groups × two orders × two reference kinds, `TestStructureJointAliasAdjacentCases`
   covers the existing-reference, mixed remove-and-rename and legal swap shapes,
   `TestIdentityDiagnosticAttributeSource` keeps the duplicate-ID location on the
   identity attribute, and `TestIdentityUnitIsPerElement` now asserts the merged
   node facts, including removal, rename, transfer and fragment aliasing.
4. **Identity provenance for diagnostics.** The reference index consumes an
   identity value only on an identity attribute (`publication.IsIDAttribute`), so
   `DUPLICATE_ID` stays attributed to the `id`/`xml:id` attribute that carries it
   and an ordinary attribute with the same value cannot move the location.

### Verification of this revision

This revision's runs are labelled by where they were taken; the product content
was committed as the fixed commit that introduces this section.

- Repository ordinary suite with the pinned EPUBCheck 5.3.0 jar, taken from this
  revision's working tree before the commit (working-tree verification, not a
  fresh checkout of the commit): all packages passed (cmd/kepub 305.943 s,
  internal/validation 256.253 s, internal/workspace 214.528 s,
  internal/publication 4.834 s, internal/xmltext 6.446 s, internal/references
  0.121 s, experiments/amp-cli 4.930 s, internal/app 0.314 s, internal/archive
  0.080 s, internal/metadata 0.455 s, internal/bookpath 0.002 s). The race suite
  on the same tree also passed all packages (cmd/kepub 498.319 s,
  internal/validation 277.581 s, internal/workspace 276.430 s,
  internal/publication 85.829 s, internal/xmltext 78.873 s, internal/references
  1.876 s, experiments/amp-cli 6.895 s, internal/app 1.899 s, internal/archive
  1.308 s, internal/bookpath 1.015 s, internal/metadata 4.616 s). `go vet ./...`
  and `gofmt` are clean on the same tree.
- The complete frozen probe set was rerun unchanged on this revision: the parent
  joint-alias probe, the parent cross-resource and IDREF probes, the medium
  identity probe and its extended file, the medium joint-identity and diagnostic
  probes, the reviewer probes for the final reference gate, IDREF coverage,
  manifest permission, attribute positives, text review shift, misplaced blocks,
  unsupported URL writes, move review, fragment identities, the schema locator
  advertisement, both R2 probes and the UTF positive control — 24 top-level test
  functions, 146 assertions including subtests, no failures and no skips, and the
  medium fuzz minimal seed `21d0b33faf043b04` replayed in ordinary and race runs.
  The joint and adjacent cases now refuse the dangling existing reference and the
  removed value in both operation orders and accept the legal swap with the
  authored bytes.
- Fuzz runs on this revision's working tree before the commit (not from a fresh
  checkout of the commit): structural edit round trip 567,343 executions in
  61.024 s, plan order independence 43,335 executions in 61.121 s, and the medium
  joint-identity target 4,466 executions in 60.053 s (with the replayed minimal
  seed), all passing with no failing input.
- Real CLI smoke on the working-tree binary built before the commit, with the
  pinned checker: joint `id`/`xml:id`
  removal and distinct renames refused a new ARIA/href reference in both
  operation orders (exit 2, no plan file), an existing frozen ARIA reference
  after a joint removal was refused (exit 1 `REFERENCE_CONFLICT`), while a legal
  joint rename and an alias transfer planned, applied and reviewed
  (`matchesExecution` true with the expected candidate bytes). A conformant joint
  positive (remove `xml:id`, rename `id`, update the reference) and the
  partial-delete survivor case were accepted by EPUBCheck 5.3.0 and exported with
  `verified: true` and the authored bytes; the two candidates that still carry
  `xml:id` on `p` were refused by EPUBCheck as non-conformant (EPUB 3.3 does not
  allow `xml:id` there), which is the checker gate and not a product defect. The
  original books' bytes were unchanged and the export ZIPs intact.
- The fixed commit, tree and bundle hashes are reported to the parent thread and
  frozen in the next batch's record.

### Acceptance of batch 2 (released)

The fixed tree `79a4548f25f9e6d7c651c48bd0d404dabe1868d0` (tree
`0a22a15cff53f4b638626968bc37dee63207a55e`, bundle
`t2-multi-operation-foundation.bundle` SHA-256
`6a35ffe0559274c0621549e99df9aad2d877bade1e82555714db353b9315a31a`) was
accepted by the independent medium review and released by the parent, who
fast-forwarded local `main` to the same commit (`origin/main` stayed at
`18618fee`, seven commits ahead, nothing pushed). The acceptance covers only the
limited schema 4 structural-editing increment: not complete T2, macOS,
power-loss or release.

- Medium (independent evidence, not inherited from the author): the frozen 24+2
  position functions and the minimal seed passed ordinary and focused race runs;
  a clean fixed product tree passed the sequential full-repository ordinary and
  race suites under the 30-minute bound with `go vet`; the 48 mixed UTF adjacency
  subcases passed ordinary and race; real CLI runs passed (ordinary 30.800 s,
  race 32.981 s) with no skips or data races. Four live fuzz targets passed in
  60 s with two workers (joint identity 2,721, block position 190,864, structural
  round trip 483,048, plan order 25,300 executions). A legal joint rename that
  removes the alias passed checker accept/export with exact ZIP bytes, and the
  candidate retaining `xml:id` was refused by EPUBCheck with RSC-005. The
  complete evidence pack `79a4548-medium-complete-evidence.tar.gz` (35,757 B,
  SHA-256 `cabeaa7f2f1f681bb81f17e37708572dc2f93edc642641ad8eb31eb389c557b6`) was
  verified, and historical failures are retained.
- Parent: all 14 received probe sources ran in their correct packages, ordinary
  and focused race with the minimal seed, no skips or data races; the real CLI
  passed in 7.390 s and 9.974 s; the effective-transaction joint-identity fuzz
  passed (994 executions in 20.132 s); after cleaning temporary probes and
  corpus the full ordinary suite passed (cmd/kepub 296.399 s, internal/workspace
  204.503 s, internal/validation 249.358 s) with `go vet` exit 0 and an empty
  product diff. The parent did not rerun the full race suite (focused race only)
  and does not claim macOS, power-loss or complete T2.

## Batch 3 — explicit-scope literal/regex batch text replacement (in progress)

Delivered: request schema 5 (`kepub-content-text-replace-v1`) with the new
`content.text.replace` v1 operation on the frozen transaction base. One explicit
element locator scopes the operation; every occurrence of a literal or Go RE2
pattern in the target subtree's own direct character data is replaced; the
expected hit count must match exactly; empty matches are refused; matches never
cross element boundaries or unwritable runs; and the full chain
plan/apply/diff/accept/history/recovery re-derives rules, hits and the write set
from the frozen baseline. `content.text.set` v1 and schemas 1–4 are unchanged.
Not delivered (remaining T2): FixProposal, ValidationDelta, cross-resource move
and OPF/nav/ID/link synchronization, font-obfuscation eligibility and explicit
old-workspace re-evaluation.

### Verification of this revision

This revision's runs are labelled by where they were taken; the product content
was committed as the fixed commit that introduces this section.

- Repository ordinary suite with the pinned EPUBCheck 5.3.0 jar, taken from this
  revision's working tree before the commit (working-tree verification, not a
  fresh checkout of the commit): all packages passed (cmd/kepub 334.677 s,
  internal/validation 282.047 s, internal/workspace 242.609 s,
  internal/publication 5.511 s, internal/xmltext 7.823 s, internal/references
  0.121 s, experiments/amp-cli 5.191 s, internal/app 0.320 s, internal/archive
  0.083 s, internal/metadata 0.524 s, internal/bookpath 0.002 s). The race suite
  on the same tree also passed all packages (cmd/kepub 557.004 s,
  internal/validation 308.143 s, internal/workspace 315.084 s,
  internal/publication 100.941 s, internal/xmltext 93.114 s, internal/references
  1.956 s, experiments/amp-cli 7.433 s, internal/app 1.933 s, internal/archive
  1.370 s, internal/bookpath 1.014 s, internal/metadata 5.833 s). `go vet ./...`
  and `gofmt` are clean on the same tree.
- Batch-3 tests: the publication layer covers literal and mixed-content
  replacement, cross-run and unwritable refusals (entity and CDATA), exact hit
  counts, empty literal/regex patterns, regex capture expansion, foreign-subtree
  scope, no-op and CRLF/UTF-16 fidelity; the workspace layer covers the
  plan/apply/review/accept chain with the pinned checker, a mixed schema 5
  transaction, refusals (hit count, empty match, head/root target, duplicate
  target, cross-run, stale revision, resource drift), a no-op write set and a
  UTF-16LE chapter end to end. The capability registry advertises the operation.
- Fuzz runs on this revision's working tree before the commit: the new
  `FuzzReplaceTextEdits` compared 967,149 executions in 60.112 s against an
  independent standard-library oracle (candidate direct text, untouched
  elements, structural verification) with no failing input; the structural round
  trip passed 875,825 executions in 61.756 s and plan order independence 35,452
  executions in 61.033 s.
- Real CLI smoke on the working-tree binary built before the commit, with the
  pinned checker: a literal replace (`One` → `Chapter One`) and a regex replace
  with capture expansion (`(Free)\.` → `$1 text.`) planned, applied, reviewed
  (`replace` with mode, expected hits, actual hits and the actual candidate
  direct text) and were accepted by EPUBCheck 5.3.0 and exported with
  `verified: true` and the expected bytes; a zero-hit operation kept an empty
  write set. Hit-count mismatch, an empty-match regex, a `head` target and a
  cross-run match were refused (exit 2, no plan file). The original book bytes
  were unchanged.
- The fixed commit, tree and bundle hashes are reported to the parent thread and
  frozen in the next batch's record.
