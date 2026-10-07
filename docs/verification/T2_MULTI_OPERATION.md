# T2 batches: multi-operation transaction foundation and XHTML structural editing

This record covers two local batches on branch `t2-multi-operation-foundation`,
which was never pushed. Batch 1 started from `origin/main`
`18618fee6b89c6ef59594c99f18f385015e0fc05` and was released by the parent at
fixed commit `23ba8f8c36b9ccf04138819acbea2af55caba832` (tree
`545f1824eb1756a979b0a554808a2bc0c0fc5c63`). Batch 2 continues from that commit
and stops at the fixed commit that introduces this section, awaiting independent
medium review and parent acceptance. Batch 2's first three trees were rejected —
`ff64188` and `f295f3f` by the high reviewer, `6f9980e` by the medium reviewer —
and every result stays bound to the reviewer and tree that produced it. Private
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

## Batch 2 — XHTML mixed content and structural editing (first three trees rejected; fixed and awaiting medium re-review)

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
2. **Counts, not flags, for the final state.** Attribute set/remove and subtree
   facts now compute their identity deltas from the element's identity set before
   and after the write; the gate keeps integer `removed`/`added` maps and derives
   `finalCount = base - removed + added` per resource and value. A new identity
   is refused while any residual frozen instance remains, a removed identity is
   only checked against frozen edges when it truly reaches zero, and a re-added
   value keeps its count so a legal partial delete plus reference stays accepted.
3. **Redundant replacement search removed.** The `replace` placement check still
   runs the precise mapped-offset byte equality and the structural parent/order
   comparison, and no longer keeps the whole-output `bytes.Contains` fallback, so
   the contract has one verification rule rather than a weaker second one.

### Verification of this revision

This revision's runs are labelled by where they were taken; the product content
was committed as the fixed commit that introduces this section.

- Repository ordinary suite with the pinned EPUBCheck 5.3.0 jar, taken from this
  revision's working tree before the commit (working-tree verification, not a
  fresh checkout of the commit): all packages passed (cmd/kepub 306.781 s,
  internal/validation 258.668 s, internal/workspace 215.969 s,
  internal/publication 4.877 s, internal/xmltext 6.402 s, internal/references
  0.127 s, experiments/amp-cli 4.903 s, internal/app 0.283 s, internal/archive
  0.094 s, internal/metadata 0.428 s, internal/bookpath 0.002 s). The race suite
  on the same tree also passed all packages (cmd/kepub 505.364 s,
  internal/validation 281.626 s, internal/workspace 281.831 s,
  internal/publication 92.144 s, internal/xmltext 78.036 s, internal/references
  1.791 s, experiments/amp-cli 7.071 s, internal/app 1.848 s, internal/archive
  1.303 s, internal/bookpath 1.016 s, internal/metadata 4.724 s). `go vet ./...`
  and `gofmt` are clean on the same tree.
- The complete frozen probe set was rerun unchanged on this revision: the two
  parent probes, the medium probe and its extended file, the reviewer probes for
  the final reference gate, IDREF coverage, manifest permission, attribute
  positives, text review shift, misplaced blocks, unsupported URL writes, move
  review, fragment identities, the schema locator advertisement and both R2
  probes, plus the UTF positive control — 19 top-level test functions, 118
  assertions including subtests, no failures and no skips. The medium identity
  probes now refuse the residual duplicate and the retained alias in both
  operation orders and accept the partial delete with a reference to the
  survivor; permanent regressions for the per-element identity unit were added
  at the publication and workspace layers.
- Fuzz runs on this revision: the structural edit round trip over UTF-8 and
  BOM-marked UTF-16 passed 828,274 executions in 61.024 s, and plan order
  independence over real structural operations passed 44,193 executions in
  61.019 s, both with no failing input.
- Real CLI smoke on the fixed binary, with the pinned checker for accept and
  export: a baseline chapter with two `id="unreferenced"` elements, deleting the
  first, inserting a new same-id element and adding `aria-labelledby` was refused
  in both operation orders (exit 2, no plan file); deleting the first and
  referencing the survivor planned, applied, reviewed (`matchesExecution` true,
  the diff showing exactly one surviving identity and the new
  `aria-labelledby`), was accepted by EPUBCheck 5.3.0 (`status: pass`, 6 checks)
  and exported with `verified: true` and the expected bytes (the existing
  single-quoted `title='old'` preserved, one `id="unreferenced"` left), with the
  original book bytes unchanged and the export ZIP intact. The earlier refusals
  were rechecked on this binary: a `headers` write plus same-transaction `id`
  removal in both orders, a lone `headers`-referenced `id` removal (exit 1
  `REFERENCE_CONFLICT`), a fragment `aria-labelledby` naming a missing identity,
  an `iframe srcdoc` fragment, a cross-resource dangling link in both orders, an
  unregistered `content.text.set` widened by a structural operation and a
  multi-token IDREF list with one missing token (all exit 2), while a no-op write
  with `expectedOldValue`, an all-present multi-token list and a fragment IDREF
  to a present identity planned.
- The fixed commit, tree and bundle hashes are reported to the parent thread and
  frozen in the next batch's record.
