# S0 P5 — bibliography classification checkpoint

This is a bounded extraction repair, not complete semantic acceptance or a
Factory Droid review. Product code and archived upstream bytes are unchanged.

## Independent failure and minimal repair

The parent's original `kepub-s0-parent-reference-tests.py` (SHA-256
`f4f42e2899aa246b7db97e0e45cc70d88ff7873f6b3fdc6b9bf0209d55ca67ab`)
was run without changing expectations. Before repair: 2 tests, 9 failures,
0.011 seconds. The CSS Snapshot's `Non-Normative References` heading contains
the substring `normative references`; the former test selected the positive
branch first. The classifier now selects explicit non-normative/informative
headings before normative headings. Both later return to normative headings
and the frozen CSS source are covered by the original independent controls.

The test is preserved verbatim as `scripts/test_epub33_parent_references.py`.
Offline index regeneration changes only the 33 CSS bibliography kinds and
their corresponding dependency citations and hashes; no citation is dropped.
The 749-row matrix remains pending. This checkpoint does not import the
parent's applicability proposal, whose historical input identity is retained.

## Executed checks

- `python3 -m unittest discover -s scripts -p 'test_*.py'`: 34 tests, 0.330s,
  OK; includes the two independent P5 tests and prior P1–P4 regressions.
- `python3 scripts/epub33_assets.py verify`: 107 assets; 749 pending,
  semanticComplete false.
- `python3 scripts/epub33_tests.py verify`: 169 cases, 170 report rows,
  170 publications, zero source gaps; execution and semantic review false.

An initial upstream verification command used the singular nonexistent name
`epub33_upstream.py` and failed (exit 2). The correct plural command and
official reproduction were subsequently executed, rather than treating this
failure as a pass. Their results are retained with the checkpoint log.

Four bibliography entries without URLs are already present in the inventory;
explicit non-URL dependency/provenance records and applicability import remain
in the next semantic increment. No legacy DTD download, ISO purchase, feature
support, conformance pass, or completed S0 gate is claimed here.
