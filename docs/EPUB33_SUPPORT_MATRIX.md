# EPUB 3.3 evidence matrix — S0 asset checkpoint

This checkpoint is **not S0 completion**, an implementation support declaration,
or closure of Issue #3. Product development remains paused. The main plan and
CLI contract are unchanged.

- [Immutable original source manifest](specs/epub-3.3/manifest.json)
- [Offline document index](specs/epub-3.3/INDEX.md)
- [Mechanical candidate/section inventory](specs/epub-3.3/inventory.json)
- [Machine-readable matrix](specs/epub-3.3/matrix.json)
- [Generated readable matrix](specs/epub-3.3/MATRIX.md)
- [Direct external references](specs/epub-3.3/external-dependencies.json)
- [Official 3.3 report/source/artifact index](specs/epub-3.3/official-tests/index.json)
- [Upstream research identities and actual licenses](specs/epub-3.3/upstreams/index.json)
- [Checkpoint verification and remaining work](verification/S0_ASSETS_CHECKPOINT.md)
- [Parent's actual 2026 checker evidence](verification/S0_EPUBCHECK_2026.md)

The mechanical inventory currently contains 749 instances: EPUB publication
408 BCP14 + 48 definition slots, reading systems 252 + 1, accessibility 40 + 0.
It preserves repeated keywords, NOT forms, enclosing text, DOM identity, fixed
document hashes and excerpt hashes. This is **not the final number of normative
constraints**. Unmarked sentences, grammar productions and deprecated
constraints still need semantic review. All five capability dimensions remain
`not-tested`; all 749 candidates and 500 core section records remain pending.
The offline `gate` command deliberately fails in this state.

`matrix.json` is the single mutable mapping source. `manualConstraints` records
additional minimum independently decidable constraints with exact archived
source provenance. `sectionReviews` records every core section/appendix, not
only sections with keyword hits. Rows distinguish applicability, phase,
five-dimensional evidence and gaps. `supported` requires evidence; a mapped
row requires applicability and phase/gap. Exclusions require reasons. The
readable matrix and plain-text extracts are generated derivatives, whose
hashes are locked in `derived.json`; rerun offline `index` after reviewed mapping
changes. Generation never marks a pending clause as reviewed.

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
