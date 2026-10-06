# S0 P6/P7 — source scope and manual provenance corrections

This is a source-matrix/verification-tool repair for independent acceptance,
not S0 completion, product implementation, conformance PASS or Factory Droid
review. Product code, three main documents, raw standards and official artifacts
are unchanged. The parent-owned acceptance record is not in this increment.

## P6: informative source observations are not independent requirements

The fixed EPUB REC 2026-01-13 has SHA-256
`f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99`.
Its §1.5 (`conformance`) makes explicitly non-normative sections, examples and
notes non-normative. These sections each have `class="informative"` and their
own “This section is non-normative” statement:

| Source section | Retained manual observations corrected | Actual normative sources kept mapped |
| --- | --- | --- |
| `sec-container-abstract-intro` (§4.2.1) | Six META-INF file-role observations | `sec-container-file-and-dir-structure`, `sec-container-metainf-inc`, and reserved-file subsections of `sec-container-metainf` (§4.2.6) |
| `sec-docs-intro` (§9.3.1) | One SMIL text/src overview observation; previously also inconsistent as mapped + S0-excluded | `sec-smil-text-elem` (§9.2.2.7), including REQUIRED child usage and src grammar |
| `sec-nav-def-types-intro` (§7.4.1) | Three toc/page-list/landmarks overview observations | `sec-nav-content-req`, `sec-nav-toc`, `sec-nav-pagelist`, `sec-nav-landmarks` (§7.2 and §7.4.2–4) |

All ten source identities, DOM paths, excerpts and excerpt hashes are retained.
Their decisions are now reasoned exclusions. Amendment import accepts an
explicit exclusion rather than unconditionally generating mapped rows.
Verification independently rejects mapped nodes inheriting informative,
note or example scope, including through nested descendants, and rejects the
mapped/S0-excluded contradiction. Pending inventories can still record these
observations without pretending review has happened.

Keyed comparison against f1f85fd proves that exactly ten rows changed decision
metadata; all other rows are identical. All 725 manual provenance records and
500 section records are unchanged. In particular the real normative container
file REQUIRED/OPTIONAL/MUST rules, text child REQUIRED, navigation-document
toc MUST, toc ordering SHOULD, and page-list/landmarks OPTIONAL constraints
are still mapped and unchanged. This proves preservation of those actual
existing records, not a new assertion that the entire standard is implemented.

Current totals: 1474 source records, **1420 mapped / 54 excluded**, no pending
rows; all five capability dimensions remain not-tested, semanticComplete false.

## P7: self-consistent manual records cannot redefine archived identity

The independent mutant changes both a manual clause and its matching row:
either a false EPUB 3.4/2027 version URI, or a real but unrelated toc section.
The actual delivered matrix did not contain these false values. Before the
repair, comparing rows only to their manual records let these mutants pass.

Manual provenance now recomputes version from manifest requestUrl, section
from the archived node's actual ancestry, and feature identity/document/hash/
DOM path from the same canonical source construction used by the inventory.
Excerpt containment and UTF-8 hash checks remain. Reviewer-assigned unmarked
levels and applicability are still semantic judgments, not inferred feature
support; source identity verification does not replace independent review.

The original parent tests are preserved byte-for-byte:

- `scripts/test_epub33_parent_informative.py`: SHA-256
  `473ec3a68978c5be7ff56d5f13cd49bcbcf3a58877aedc21d9254d6d29e68c09`.
- `scripts/test_epub33_parent_manual_provenance.py`: SHA-256
  `cd3831d5fb127aeabd351e559d174472e93ad3eb23bbdc1c89673a12d06588dd`.

## Actual failures and regression results

- P6 raw original: 2 tests, 10 failed subtests, 0.633s. After source corrections:
  2/2 PASS, 0.643s; actual normative REQUIRED/SHOULD positive controls pass.
- An intermediate P7 invocation appeared green while the still-unintegrated
  P6 matrix failed the new scope guard. This was not accepted as P7 proof.
  After completing P6 and verifying its valid matrix, the original P7 test
  reproduced independently: 1 test, 2 failed subtests, 7.979s. After the manual
  canonical-binding fix: 1/1 PASS, 7.792s. Both logs, and the intermediate
  unrelated-rejection result, are retained.
- First 66-test regression: 104.770s, exit 1, two fixture setup errors. The new
  informative fixtures tried to replace an already frozen fake upstream and
  correctly hit the archive drift guard. Tests now use separate fresh temporary
  archives; the archive safety rule and parent expectations were not weakened.
- Four targeted source-scope/atomic-import/provenance regressions: 4/4 PASS,
  0.045s. Full final Python regression: **66/66 PASS, 102.842s, exit 0**.
  Includes all unchanged independent P1–P7 controls, nested informative scope,
  explicit excluded amendments, mapped/excluded-phase rejection and mirrored
  version/section/feature-ID mutations against fixed source identities.
- Source and semantic verify: exit 0, 107 assets / 1474 rows / 283 supporting
  sections / 63 CSS modules. Official verify: 169 cases / 170 report rows /
  170 publications / no source gaps; behavior remains unexecuted. Upstream
  verify: 14 projects / one deferred adoption-blocking license gap, not legal
  approval. No new Go combination or formal-checker run was requested here.

## Deterministic fixed-tree handoff

The prior a330430 first document replay did drift in four files; its original
delta is retained, not replaced by later idempotence. The f1f85fd increment
commits the exact documented packet order and is included when handing this
repair off relative to a330430. The unchanged full recipe in
`EPUB33_SUPPORT_MATRIX.md` rebuilds both corrected mappings and derivatives;
replay/commit byte equality is recorded in the handoff evidence. Neither an
offline verify nor a no-drift rebuild closes S0 or authorizes T1.

Next: parent independent repair acceptance, then a fixed combined-tree real
Factory Droid demo/review including its latest actual acceptance record.
No S0 Droid request, push, release or product implementation has started here.
