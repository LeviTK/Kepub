# T1b local navigation-structure certainty checkpoint

This bounded increment starts at `ac98b6135ae44cbf06762ec4e8dcdb30319a12e1`,
not origin/main. It fixes audit F2; it is not independent approval, parent
acceptance, or permission to start T2. Parent-owned contracts are unchanged.

The XML index records direct-child uncertainty separately from descendant
content uncertainty. It derives both from retained expansion-source intervals,
not from a literal `&name;` search, an attribute flag, or document-wide partial
status. Known direct non-whitespace is recorded independently: known lexical
pieces beside an unresolved placeholder are decoded separately, so removing
the placeholder cannot introduce syntax or erase real bare text.

Navigation uses the uncertainty of the structure actually being checked.
Unknown children cannot establish missing items, labels, text or content;
known duplicates and illegal children remain diagnostic. An unknown label
does not suppress a known href/src. Unknown TOC identity or indispensable root
structure can remain blocked without a fabricated missing-structure error.
Existing XML_ENTITY_UNRESOLVED and XML coverage remain; no schema fields,
external reads, writable spans, checker exceptions or budget changes are added.

## Executed checks before packaging

- Independent regression table: 18 nav/NCX controls. Before the navigation
  repair, 12 assertions failed (actual test exit 1); the original log remains.
- Shared index: 10 asymmetric content controls across UTF-8 and BOM UTF-16
  LE/BE. Includes nested unknown expansion, known entity-looking literal text,
  CDATA, descendant-only uncertainty and unknown attributes with known content.
- Three-package normal suites: actual exit 0. An earlier command incorrectly
  included nonexistent `internal/content`; its setup failure is retained.
  Content behavior belongs to publication, which passed in both invocations.
- Three-package race suites: actual exit 0 (xmltext 91.454 s, publication
  102.193 s, metadata 10.735 s); targeted vet: actual exit 0.
- New local-certainty fuzz: actual exit 0, 32,789 executions, 31.052 s.
- Parent's original 13 synthetic EPUBs, not regenerated substitutes: all 13
  actual CLI assertions passed. Real known bare text beside an unknown entity
  still reports NAVIGATION_STRUCTURE. Original book bytes remain unchanged.

Private logs and actual-exit records are retained in the durable evidence
directory and transferred separately. The old ac98 checks-v4 continues on its
unchanged checkout; neither its results nor the rejected 11a0 audit approve
this new tree. New-tree full normal/race/vet, source/encoding/local fuzz and
real-checker/original-book regressions must be recorded against the fixed
checkpoint separately. Parent integration and new independent audit remain.
