# S0 asset checkpoint (not the S0 completion gate)

## Ownership and frozen input

Work is isolated from the coding Orb's preserved T1 draft. Input baseline is
parent local commit `e8e8b2a539fa61e46cc95d5940b36960e3462e8f`, transferred in the
actual full-history bundle with SHA-256
`0084fb34b0e40d4970d573154f9d875dde2a11b3bf324694805d26ce30353f64`.
Parent item 5 and its full-regression record were imported from verified bundles
and cherry-picked as `f53910c` and `af491df`; only the parent-owned new validation
test and its report were integrated. Product code, go.mod/go.sum, setup and the
three main documents were not edited. No push, release or issue closure.

## Actual captures

- Source archive: 104 assets, 12,549,875 raw bytes, zero unresolved manifest
  failures. Four fixed baselines, seven supporting notes, XML Fifth Edition,
  Namespaces Third Edition, HTML XML chapter, errata and actual issue/comments,
  3.3 implementation/report pages, display dependencies and linked schema files.
  Errata API capture separates 17 confirmed and 3 raised issues; pending issues
  are not treated as normative changes. Included ReSpec Markdown bodies are
  archived explicitly; no page script is executed to substitute for source.
- Direct references: 336 citation records / 227 distinct URLs, not recursively
  expanded. Stage applicability is still pending. Undownloaded sources have
  null hashes and are not represented as full-text captures.
- Official report: 169 unique IDs / 170 rows, retaining the repeated
  `pkg-unique-id` occurrence. Report commit is
  `54092b4233253e9aac80e93ec4782b380b4b3403`. Every official EPUB's complete
  uncompressed file inventory matches an archived historical source tree;
  ZIP container hash identity is not assumed. Local generator sorts entries,
  stores mimetype first/uncompressed, and fixes ZIP timestamps. Original
  binaries, source tar files, generated binaries and separate hashes are kept.
  Offline reproduction records exact Python/zlib versions and the generation
  function hash; it verifies the generated ZIP hash rather than merely saying
  the source looks equivalent. No official EPUB is executed or rendered here.
- The first two official fetch failures are retained as
  `initialCaptureFailures`. `ocf-font_obfuscation-bis` requires the real source
  directory `_bis`; complete content and identifier resolve it, not a guessed
  alias. `cnt-css-fonts` was split/deleted in commit
  `4f5a851bcf35fda29ff188795eacde4e054d0371`: the original historical binary
  matches source commit `bc8b8e86c6d34fd10cb2f0504c8be01fc03182ff`. It is not
  replaced with four newer cases. Original 404 evidence remains in the index.
- 14 research repositories / five role groups have actual public commits and
  metadata. 13 actual license files are archived. daisy/pipeline-cli-go's license
  API returned 404: source identity is present, adoption remains blocked.
  NOASSERTION is not changed into an invented SPDX identifier. This deferred
  adoption gap does not block research indexing; legal advice, dependencies,
  machine protocol and behavior are not established by this capture.

Raw upstream bytes retain copyright/license text, examples and diagrams.
Associated display assets are documentation assets, not newly embedded EPUB
fonts. Original script files are data; the tooling never runs them. This is a
bounded source archive, not a browser implementation or offline preview.

## Reproduction commands

Run from the repository root, with Python 3 standard library and curl available:

```sh
python3 -m unittest discover -s scripts -p 'test_*.py' -v
python3 scripts/epub33_assets.py verify
python3 scripts/epub33_tests.py verify
python3 scripts/epub33_tests.py reproduce
python3 scripts/epub33_upstreams.py verify
python3 scripts/epub33_assets.py gate  # expected exit 1 while review pending
```

`index` is offline and preserves existing matrix decisions. `fetch` with an
existing manifest checks upstream identity and never silently accepts drift or
overwrites modified local bytes. A new archive uses a separate root and explicit
`fetch --bootstrap`, then `index`, then human review of its diff. First capture
uses a 300-resource / 128 MiB source-archive budget and 32 MiB per download. This
does not change product XML or search budgets. Official EPUB sources/artifacts
are a separate finite report set, not a recursive Web traversal.

For an independently reviewed upgrade/reproduction of the official mapping:

```sh
git clone https://github.com/w3c/epub-tests /tmp/epub-tests-source
git -C /tmp/epub-tests-source checkout 54092b4233253e9aac80e93ec4782b380b4b3403
python3 scripts/epub33_tests.py capture --root NEW_ARCHIVE_ROOT --source-repo /tmp/epub-tests-source
# First capture records the two historical gaps; resolve explicitly after review:
python3 scripts/epub33_tests.py resolve-missing --root NEW_ARCHIVE_ROOT --source-repo /tmp/epub-tests-source
```

`NEW_ARCHIVE_ROOT` first needs matching captured report HTML. New snapshots do
not overwrite this evidence. Future source availability is not guaranteed;
download failures and upstream drift fail explicitly.

## Executed checks and limits

18 offline asymmetric/adversarial tests pass: missing source, wrong hash,
fixed-version drift despite updated byte hash, dynamic shell, included body,
missing matrix mapping, hiding a candidate from both inventory and matrix,
upstream drift without overwrite, optional HTML closing tags, missing section
review, unsupported support claim, derivative corruption/omission, official
report omission, duplicate tar entry, deterministic ZIP and stored mimetype.
An omitted CSS import also fails independently of the declared dependency list.
An independent red test exposed omission of the same candidate from inventory
and matrix; verification now independently re-extracts the archived inventory.

Actual offline outputs: assets `104 / pending749 / semanticComplete:false`;
official sources `169 / sourceGaps:0 / executed:false`; upstream research
`14 / gaps:1 / behaviorTested:false`. `gate` exits 1 with an explicit pending
clause/section error. This expected rejection is not called S0 PASS.

The isolated combined parent-test tree was run with Go 1.27.1,
`go test ./...`, `go test -race ./...`, `go vet ./...` (GOPROXY=off), all exit 0.
Normal CLI103.673s/workspace131.842s/validation195.817s;
race97.575s/148.957s/194.896s; vet no diagnostics. These include real fixed
EPUBCheck5.3.0 parent tests; they are not tests of later XML/CLI features, nor
execution of the 169 official reading-system cases. Parent's separate results
remain in S0_EPUBCHECK_2026.md, not replaced with this Orb's timings.

Remaining S0 work: semantic review of all 500 core section records and 749
mechanical instances; add minimum unmarked/grammar/deprecated constraints;
assign direct-dependency stages; map each official case to fixed REC clauses
and appropriate object/phase; review CSS/supporting-note chapter/module
inventory. All five dimensions are currently not-tested. No full S0 Droid
demo/review has been started. It follows the stable complete matrix, parent's
independent acceptance, and a fixed combined tree. T1 remains paused.
