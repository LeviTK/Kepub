# EPUB 3.3 evidence matrix — S0 semantic checkpoint

This source checkpoint alone is **not S0 approval**, an implementation support
declaration, or closure of Issue #3. Actual review decisions and phase status
are recorded in the [parent acceptance record](verification/S0_PARENT_ACCEPTANCE.md).
T1 requires current-input parent and actual Droid approval through the offline
`gate`; source counts and successful generation do not grant that approval.

- [Immutable original source manifest](specs/epub-3.3/manifest.json)
- [Offline document index](specs/epub-3.3/INDEX.md)
- [Mechanical candidate/section inventory](specs/epub-3.3/inventory.json)
- [Machine-readable matrix](specs/epub-3.3/matrix.json)
- [Generated readable matrix](specs/epub-3.3/MATRIX.md)
- [Source-addressed semantic review packets](specs/epub-3.3/reviews/)
- [Reproducible official/supporting semantic index](specs/epub-3.3/semantic-index.json)
- [Direct external references](specs/epub-3.3/external-dependencies.json)
- [Official 3.3 report/source/artifact index](specs/epub-3.3/official-tests/index.json)
- [Upstream research identities and actual licenses](specs/epub-3.3/upstreams/index.json)
- [Checkpoint verification and remaining work](verification/S0_ASSETS_CHECKPOINT.md)
- [Stable semantic increment and actual checks](verification/S0_ASSETS_SEMANTICS.md)
- [P6/P7 scope and provenance corrections](verification/S0_ASSETS_SCOPE_PROVENANCE.md)
- [R1 gate, repository evidence and source-only status repairs](verification/S0_ASSETS_R1_GATE_EVIDENCE.md)
- [R2 source reconciliation and algorithm-scope correction](verification/S0_ASSETS_R2_CORRECTIONS.md)
- [Parent's actual 2026 checker evidence](verification/S0_EPUBCHECK_2026.md)

The mechanical inventory contains 749 instances: EPUB publication
408 BCP14 + 48 definition slots, reading systems 252 + 1, accessibility 40 + 0.
It preserves repeated keywords, NOT forms, enclosing text, DOM identity, fixed
document hashes and excerpt hashes. This is **not the final number of normative
constraints**. The source review currently adds 798 manual instances, for 1547
total rows: 1484 mapped + 63 reasoned exclusions. All five capability dimensions
remain `not-tested`; all 500 core section records have source-review notes.
The bounded Amp research drafts were corrected against the parent's independent
attribute/default/landmarks tests and coding-Orb source review. P6 additionally
retains ten informative-source observations as exclusions, not requirements;
P7 binds manual version/section/identity fields directly to the fixed archive.
The corresponding actual normative constraints remain mapped. Source-review
notes alone are not independent acceptance or a successful Factory Droid review.
R1 and R2 completed with findings; their raw evidence and reading limitations
are retained, not promoted to approval. Subsequent decisions belong in the
parent acceptance record. `semanticComplete` stays false;
the offline `gate` aggregates source, derivative, semantic, official-fixture
and upstream validators and requires current-input parent and actual Droid
approval records. Editing that boolean cannot close S0. Counts never establish
normative completeness by themselves.

`matrix.json` is the single mutable mapping source. `manualConstraints` records
additional minimum independently decidable constraints with exact archived
source provenance. `sectionReviews` records every core section/appendix, not
only sections with keyword hits. Rows distinguish applicability, phase,
five-dimensional evidence and gaps. `supported` requires evidence; a mapped
row requires applicability and phase/gap. Exclusions require reasons. The
readable matrix and plain-text extracts are generated derivatives, whose
hashes are locked in `derived.json`; rerun offline `index` after reviewed mapping
changes. Generation never marks a pending clause as reviewed.

[Implementation references](specs/epub-3.3/reviews/implementation.json) link 13
specific existing clauses to related Go code, named tests and the actual narrow
2026 checker record, with explicit untested boundaries and an identified
same-version multi-rootfile gap. All capability states remain `not-tested`:
references alone are not execution or clause-wide support. Offline review import
reapplies these associations deterministically. Evidence files and named Go
tests must exist inside the repository; `supported` additionally requires a
clause/test/dimension-specific passed execution record, never a code path alone.

Offline reproduction of the current source mapping:

```sh
python3 scripts/epub33_assets.py index \
  --review docs/specs/epub-3.3/reviews/kepub-s0-epub-semantic-r2.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-rs-semantic-draft.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-a11y-semantic-draft.json \
  --amendment docs/specs/epub-3.3/reviews/publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r1-publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r1-accessibility-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r2-publication-amendments.json \
  --amendment docs/specs/epub-3.3/reviews/r2-rs-amendments.json
python3 scripts/epub33_semantics.py index
python3 scripts/epub33_assets.py verify
python3 scripts/epub33_semantics.py verify
```

Review import verifies canonical source fields, source excerpts, all candidate
identities and section coverage before writing matrix bytes. Missing mappings
or invented excerpts cannot partially replace the matrix. Amendment selectors
retain independent value/default/alias/list-property instances and recompute
their UTF-8 excerpt hashes. They never fabricate behavioral evidence.
After all reviews and amendments are merged, import must retain every existing
feature identity; mistaken observations remain as reasoned exclusions, not
silent deletions. For sections marked complete, independent frozen-DOM checks
reconcile formal algorithm steps/branches and 27 explicitly reviewed list/value/
definition families. A wider ancestor, another sibling or an Explanation cannot
replace a missing member; multiple statement paragraphs remain distinct.
These bounded structural checks do not classify all natural-language obligations
or prove semantic completeness. Pending sections still block the aggregate gate.

The semantic index includes 169 official cases / 170 required publications,
explicit source correspondences for 12 obsolete report anchors, 283 supporting
section records (243 indexed sections + 40 flat CSS headings) and 63 CSS modules.
Historical URLs, report expectations and levels are retained, not silently
replaced. Initial draft targets/gaps remain labelled historical; current
correspondences are source-addressed and preserve conditional/optional scope.
CSS tier membership is checked against actual source lists. Flat Bikeshed
sections carry explicit sibling ranges; display excerpts ending in an ellipsis
are explicitly truncated derivatives, not full-source hashes.

R1 source corrections register 59 inherited leaf conditions/options with their
exact introductory context. AND ordering, any-of page-navigation triggers,
one-of fallback alternatives, OPTIONAL objectives, namespace exclusions and
iframe exceptions are distinct; none is a blanket mandatory feature. Value
definitions bind their actual preceding `dt` to the mapped `dd`, without
creating obligations from labels. Nine explicitly exemplary pseudo-code
identities remain excluded. P9 corrects R1's overbroad exclusion of two font
definition paragraphs: their original identities are restored to mapped, with
the third XOR definition added. REC §1.5's named `details/summary Explanation`
blocks are distinct from normative algorithm definitions and cannot be mapped
as requirements. Compression-order/key/specifying MUSTs remain mapped.
The R2 amendments add 14 source identities, including both OCF URL success
branches, the filename/path loop condition, empty-property and alternate/nav
definitions, and conditional RS algorithm grouping/branch statements. All 1533
previous identities remain. Neither equivalent-result algorithms nor optional
reading-system behavior requires literal example-code execution or new CLI/UI
features. Historical review notes and superseded amendments remain visible;
the final amendment order above supplies the corrected current mapping.

The three fixed RECs' raw `data-tests` attributes are independently reconstructed:
237 fragment references / 162 IDs, 158 known official cases; four unresolved
IDs remain explicit and eleven cases without a REC backlink retain their report
targets. Concrete normalized contexts add missing backlinks without replacing
publication-side conditions. All 456 external structural-test links are marked
external and unexecuted, not silently treated as the 169 RS fixtures.

The archive now contains 118 files, preserving the original 107 files byte-for-byte. Three
directly linked NVDL dispatchers from fixed EPUBCheck commit
`029831b8f477e4519e9734c984ee24357547a698` have a deduplicated closure of 14 schema
files plus the actual directory IDPF MIT-form LICENSE. NVDL validate/schema,
Schematron include/href and inherited XML base are checked offline. This is
source closure, not actual execution of those schemas or whole-REC support.

External-reference review is now explicitly rebound to the corrected source:
227 URL records (332 bibliography entries) plus four independent non-URL
records cover all 336 source bibliography entries. The original parent proposal
and its original input bytes/hashes remain in `reviews/`; the generated index
records every one of the 33 P5 citation-kind corrections with source hash/DOM.
The four non-URL entries retain their exact titles/editions, null target URLs
and hashes, and undownloaded status; title matching never merges them into a
different edition. `citationSourceCapturedAt` refers to the citing page's
capture, not acquisition of the target external document. The registry's
undownloaded entries retain null hashes; T3's NCX/DTBook/XHTML1.1 legacy offline
assets are prerequisites, not claimed downloaded. Paid ISO/ANSI full text is a
later-stage gap, not an S0 purchase. Later module semantics, rendering,
accessibility, legal applicability and adoption remain explicit gaps. Source
mapping is ready for independent acceptance; this checkpoint does not unblock
T1 or replace the subsequent fixed-tree Factory Droid review.

[Finite T3 prerequisite registry](specs/epub-3.3/reviews/t3-prerequisites.json)
registers OPF/OPS/OCF 2.0.1, NCX, DTBook, XHTML1.1 DTD/modules/entity sets.
Every target remains pending with null acquired hash/path. The first three
retain actual fixed bibliography URLs; exact remaining editions/URIs and their
finite offline dependency closure must be frozen at T3, not guessed from EPUB3
Appendix B. Registry presence does not authorize runtime downloads or migration.

## Explicit boundary register for semantic review

| Boundary | Source and meaning | Planned resolution / present evidence |
| --- | --- | --- |
| CSS UTF-16 | EPUB §6.2.1 allows UTF-8/UTF-16 for CSS; XML decoding alone is not CSS support. | T5 must independently test CSS byte decoding, token locations and preservation; no support claimed here. |
| XML resource profiles | EPUB §3.9 applies to XML-based publication resources; OCF META-INF XML and package OPF have additional own vocabulary/schema constraints. Appendix B tuples do not authorize XHTML or OPF external subsets. | T1a/T1b must carry resource/version context; META-INF/OPF cannot share an unqualified document-type guess. |
| NOTATION | XML Fifth Edition §4.7 defines identifiers naming a notation, distinct from external entity declarations; EPUB §3.9 forbids external entity declarations, not every occurrence of a URI in DTD. Reading Systems §15.3 says SHOULD NOT resolve DOCTYPE/ENTITY/NOTATION external identifiers. | Never fetch such identifiers. T1b must freeze full legal internal-subset semantics and provenance; no parser acceptance is claimed by this research note. |
| EPUB2 external DOCTYPE | The EPUB3 Appendix B restriction is version/profile-specific. Legal EPUB2 input that needs unsupported DTD/entity semantics is not automatically malformed EPUB3. | T3 must archive OPF/OPS/OCF 2.0.1, NCX/DTBook, XHTML1.1 DTD/modules/entities and transitive offline whitelist assets before migration. Those assets are not claimed downloaded by S0. |
| Old workspace states | A new validation evidence/qualification model cannot retroactively fabricate checks or silently replace durable source identity. | T2 must explicitly re-evaluate old state formats and preserve previous evidence/history. No migration or state-schema change in this checkpoint. |

The final semantic pass must map applicable clauses to existing code/tests or
explicit gaps and to T1–T6 / S2–S4 without reducing the frozen standard target.
RS rendering/accessibility requirements remain in the matrix even though they
are outside the currently authorized CLI implementation. Archive counts,
generated EPUBs and upstream license records are not behavioral, rendering,
accessibility, legal or conformance PASS evidence.
