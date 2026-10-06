# S0 P4 — required paired publication

This is an archive correction, not S0 acceptance or a reading-system PASS.
The parent's independent paired-fixture test first failed on the 169-artifact
index: `Duplicate report ID must not discard the required second publication`.
That failure is preserved in the source Orb's `.agents/kepub-s0-p4-red.log`.

The fixed report contains 169 unique case IDs and 170 report rows. The repeated
`pkg-unique-id` case needs two publications, not one duplicated index record.
Its required second publication is `tests/pkg-unique-id_duplicate` at report
commit `54092b4233253e9aac80e93ec4782b380b4b3403`, source tree
`1bb88991c92b28e1cbfc49e85eb91070ff864965`. The actual website binary is
`https://w3c.github.io/epub-tests/tests/pkg-unique-id_duplicate.epub`:
1976 bytes, SHA-256
`2f1b1967d196d94ed6e330bfe037e7eff44447d5802fbe2158de30c829cb09b7`.
No binary URL at that Git commit is claimed.

The nested `pairedFixtures` record retains source tar, website ZIP and generated
ZIP paths and hashes, the complete five-file content inventory, source/report
commit and generator runtime. All five original/source/generated uncompressed
files match. The 169 cases now have **170 required publications**. Old archives
and binaries were not replaced. `capture-pairs` adds this required evidence to
an existing snapshot; initial capture also includes it. Frozen pairs are not
redownloaded. Offline verify/reproduce check pairs as well as primary artifacts.

## Executed checks

- Parent's original `test_epub33_parent_pair.py`: 1/1 PASS after the correction;
  its independently selected five-file expectations are unchanged.
- Four additional offline mutations reject deleted pair records, forged source
  commit, forged generated hash and missing source file.
- Combined Python suite at this working stage: 38 tests, exit 0.
- Official verify: 169 cases / 170 report rows / 170 publications / 0 source
  gaps; `semanticReviewComplete:false`, `executed:false`.
- Official offline reproduce: exit 0, including the pair.

The two-book interaction has not been run in a reading system. Source/artifact
integrity does not establish that either book is displayed independently.
Full semantic mapping, parent acceptance and the fixed-tree Factory Droid
S0 review remain separate gates. T1 is still paused.
