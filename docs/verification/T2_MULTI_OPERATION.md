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

## Batch 2 — XHTML mixed content and structural editing (rejected first tree, fixed and awaiting re-review)

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
as passes; the defect was transport, not product. The reviewer's clean full
suites on the rejected tree passed (ordinary cmd/kepub 309.699 s,
internal/workspace 216.502 s, internal/validation 259.478 s; race cmd/kepub
500.342 s, internal/workspace 266.523 s, internal/validation 276.432 s; vet and
`git diff --check` clean), and a passing suite does not offset the ten probe
failures.

### Fixes in this revision (minimal, one rule per cause)

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
  documented fallback when another operation inserts into that context), and a
  count guard refuses a silent skip.
- Attribute writes escape both quote characters, accept empty value intervals,
  and emit namespace declarations with correct separators and a reused or free
  prefix.
- Fragments refuse dynamic content and unsupported URL attributes and require
  legal XML names for identities.
- `xhtml.element.move` advertises a locator for its anchor.

### Second rejection (this round) and reviewer handover

The corrected tree `f295f3f` was itself rejected by the independent high review
after the parent reproduced its findings. Two blocker groups remained, and the
high reviewer's clean suites on that tree (ordinary `cmd/kepub` 309.699 s,
`internal/workspace` 216.502 s, `internal/validation` 259.478 s; race
`cmd/kepub` 507.167 s, `internal/workspace` 275.164 s, `internal/validation`
279.907 s; vet and `git diff --check` clean) do not offset them:

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

### Verification of this revision

- The parent probe and all ten reviewer probes now pass, with their expectations
  unchanged; permanent regressions were distilled from them (cross-resource and
  same-resource order independence, identity synchronization positives, IDREF
  gate, manifest permission boundary, attribute positives, review target
  mapping, misplaced-block refusals, fragment identity and dynamic-content
  refusals, schema locator advertisement).
- Targeted fuzz targets were added and actually run: physical interval
  invariants (943,012 executions in 61 s), structural edit round trip over UTF-8
  and BOM-marked UTF-16 (1,464,358 executions in 61 s after the positional
  verification), and plan order independence over real structural operations
  (48,047 executions in 61 s). All three passed with no failing input.
- The frozen probe set was rerun unchanged on this revision: the parent probes
  for cross-resource links and new IDREFs, the reviewer probes for the final
  reference gate, IDREF coverage, manifest permission, attribute positives, text
  review shift, misplaced blocks, unsupported URL writes, move review, fragment
  identities, the schema locator advertisement and both R2 probes all pass. The
  attribute byte controls were additionally reproduced across UTF-8, UTF-16LE and
  UTF-16BE at the publication layer, including the taken-prefix case.
- Repository ordinary suite with the pinned EPUBCheck 5.3.0 jar: all packages
  passed (cmd/kepub 307.456 s, internal/validation 260.150 s,
  internal/workspace 213.876 s, internal/publication 4.674 s,
  internal/xmltext 6.465 s, internal/references 0.139 s). Race suite: all
  packages passed (cmd/kepub 499.009 s, internal/validation 274.137 s,
  internal/workspace 271.587 s, internal/publication 88.984 s,
  internal/xmltext 80.023 s, internal/references 1.861 s). `go vet ./...` and
  `gofmt` are clean.
- Real CLI smoke on the fixed binary: a schema 4 plan with an attribute write, an
  insertion and a deletion applied, reviewed (`matchesExecution` true with the
  actual candidate values), accepted by the pinned checker and exported with the
  expected bytes (apostrophe escaped inside a single-quoted value, existing
  `epub` prefix reused, identity removed). The same smoke refused a
  cross-resource dangling link in both operation orders (exit 2), an
  unregistered-resource `content.text.set` widened by a structural operation
  (exit 2), a `headers` IDREF removal (exit 1 `REFERENCE_CONFLICT`) and an
  `iframe srcdoc` fragment (exit 2); a move with a locator anchor planned and its
  review reported `block bytes preserved`. For this revision the smoke also
  refused a new `headers` IDREF pointing at an identity removed in the same
  transaction in both orders, a fragment `aria-labelledby` naming a missing
  identity, and a multi-token IDREF list with one missing token (all exit 2),
  while an existing-identity no-op write, an all-present multi-token list and a
  fragment IDREF to a present identity planned, reviewed with their actual
  values, passed the pinned checker and exported.
- The fixed commit, tree and bundle hashes are reported to the parent thread and
  frozen in the next batch's record.
