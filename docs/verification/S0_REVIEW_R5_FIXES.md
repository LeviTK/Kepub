# Bounded R5 F1/C5 source corrections

Baseline: accepted S0 implementation 4bb5cc4, not origin/main. This increment
does not constitute complete-S0 review or acceptance and does not unlock T1.

Two source-hash-bound amendment packets add exactly eight direct records:
EPUB contributor p2 inheritance, identifier p3 persistence advice, date p3
additional-date advice, subject p1 label/code advice, last-modified p3 update
advice, core-media-types ul1/li1/p2 ordered preference advice, privacy p7
conditional DRM advice; RS CSS p4 developer support/documentation advice.
The seven lowercase advisory records remain UNMARKED ADVISORY (or conditional
advisory), not BCP14 MUST/SHOULD. Five capability dimensions remain not-tested.
Contributor p1's secondary-role boundary is retained in the review reason and
section note; p2 refers to existing creator requirements in all other respects,
not a duplicate mandatory creator-role family. RS advice is allocated to S2-S4.

Finite reconciliation selects these eight frozen source members independently
of surviving rows. It is not a general natural-language classifier. Regression
controls remove each member or attempt to replace it with a wide ancestor or
different sibling, with direct-member and truthfully pending-section positives.
Multi-amendment import failure is checked to leave matrix bytes unchanged.

The parent's original test is installed byte-for-byte at
scripts/test_epub33_parent_r5_inheritance.py (SHA256
682ab95860030d86de36c18e26910466556614c1e7c828f05d4aebc502df608b).
Local original-baseline red: 3 tests / 11 failures / 22.456 s. Final targeted
red/green group: 6 tests passed in 28.206 s. Logs persist outside the input tree
in /home/user/kepub-s0-durable/r5-fix-evidence.

Two intermediate failures are preserved: using list-only contextDOM binding for
a paragraph was rejected before any write (no verifier relaxation added); one
own mutation test inadvertently selected the subject statement itself as its
"sibling", then was corrected to select a different source sibling. Neither is
represented as a successful product check or hidden by the final targeted pass.

All 1598 old rows and 849 manual constraints are byte-value-identical by identity.
New totals: 1606 rows, 857 manual, 1543 mapped, 63 excluded; all 500 section
identities retained, only eight review notes/reviewer metadata changed. Original
assets, official/upstream artifacts, Go product and parent documents are unchanged.
C3/M1 observations, non_normative behavior and approval input scope are untouched.

## Offline rebuild recipe addition

Append these two arguments after the existing three reviews and six amendments:

```sh
--amendment docs/specs/epub-3.3/reviews/r5-publication-amendments.json \
--amendment docs/specs/epub-3.3/reviews/r5-rs-amendments.json
```

Then run semantics index, the four verifiers, official reproduce, full Python
suite and both tracked/staged diff checks. Normal gate must still reject missing
independent acceptance. Full-suite/first complete replay results are delivered as
separate evidence after actual completion on the fixed increment HEAD; targeted
passes do not assert that those still-pending checks have passed.
