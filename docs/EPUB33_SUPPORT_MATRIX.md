# EPUB 3.3 evidence matrix — S0 semantic checkpoint

This checkpoint is **not S0 completion**, an implementation support declaration,
or closure of Issue #3. Product development remains paused. The main plan and
CLI contract are unchanged.

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
- [Parent's actual 2026 checker evidence](verification/S0_EPUBCHECK_2026.md)

The mechanical inventory contains 749 instances: EPUB publication
408 BCP14 + 48 definition slots, reading systems 252 + 1, accessibility 40 + 0.
It preserves repeated keywords, NOT forms, enclosing text, DOM identity, fixed
document hashes and excerpt hashes. This is **not the final number of normative
constraints**. The source review currently adds 725 manual instances, for 1474
total rows: 1420 mapped + 54 reasoned exclusions. All five capability dimensions
remain `not-tested`; all 500 core section records have source-review notes.
The bounded Amp research drafts were corrected against the parent's independent
attribute/default/landmarks tests and coding-Orb source review. P6 additionally
retains ten informative-source observations as exclusions, not requirements;
P7 binds manual version/section/identity fields directly to the fixed archive.
The corresponding actual normative constraints remain mapped. These are not
independent acceptance or Factory Droid results. `semanticComplete` stays false
and the offline `gate` deliberately fails until the remaining S0 review gates
are closed. Counts never establish normative completeness by themselves.

`matrix.json` is the single mutable mapping source. `manualConstraints` records
additional minimum independently decidable constraints with exact archived
source provenance. `sectionReviews` records every core section/appendix, not
only sections with keyword hits. Rows distinguish applicability, phase,
five-dimensional evidence and gaps. `supported` requires evidence; a mapped
row requires applicability and phase/gap. Exclusions require reasons. The
readable matrix and plain-text extracts are generated derivatives, whose
hashes are locked in `derived.json`; rerun offline `index` after reviewed mapping
changes. Generation never marks a pending clause as reviewed.

Offline reproduction of the current source mapping:

```sh
python3 scripts/epub33_assets.py index \
  --review docs/specs/epub-3.3/reviews/kepub-s0-epub-semantic-r2.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-rs-semantic-draft.json \
  --review docs/specs/epub-3.3/reviews/kepub-s0-a11y-semantic-draft.json \
  --amendment docs/specs/epub-3.3/reviews/publication-amendments.json
python3 scripts/epub33_semantics.py index
python3 scripts/epub33_assets.py verify
python3 scripts/epub33_semantics.py verify
```

Review import verifies canonical source fields, source excerpts, all candidate
identities and section coverage before writing matrix bytes. Missing mappings
or invented excerpts cannot partially replace the matrix. Amendment selectors
retain independent value/default/alias/list-property instances and recompute
their UTF-8 excerpt hashes. They never fabricate behavioral evidence.

The semantic index includes 169 official cases / 170 required publications,
explicit source correspondences for 12 obsolete report anchors, 283 supporting
section records (243 indexed sections + 40 flat CSS headings) and 63 CSS modules.
Historical URLs, report expectations and levels are retained, not silently
replaced. Initial draft targets/gaps remain labelled historical; current
correspondences are source-addressed and preserve conditional/optional scope.
CSS tier membership is checked against actual source lists. Flat Bikeshed
sections carry explicit sibling ranges; display excerpts ending in an ellipsis
are explicitly truncated derivatives, not full-source hashes.

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
