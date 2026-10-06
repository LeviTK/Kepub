# S0 asset checkpoint: parent negative-test corrections

This is a correction increment, **not S0 acceptance**. Parent independently
tested checkpoint `b92e190` and reported three real failures. The original
checkpoint's archive checks were insufficient; its 104-file count must not be
interpreted as a complete normative asset package. Parent owns the separate
acceptance record; this file does not edit or supersede it.

## Reproduction and corrections

The received parent script is preserved byte-for-byte as
`scripts/test_epub33_parent_acceptance.py`, SHA-256
`ec975736bfec2e23369d44c40ba49eda450b4c1bb96417000c834eddb71e7dfe`.
Parent's original three-FAIL log has SHA-256
`d59b019115416cb68d2bdfc3e2e78abce7bff397d1b19795197b0886f0f5a152`.
On this Orb's later `dc7d603` tree before these changes, the same script returned
one PASS / two FAIL: sourceCommit-null was already rejected by the subsequent
independent-red fix, but schema closure and redirected REC identity were still
real failures. This distinction does not invalidate the parent's b92 results.

1. **Official source completeness.** Verification now requires every case's
   commit, source path/archive/hash, generated artifact/hash, and an original
   website or historical artifact/hash. Torn optional artifact path/hash pairs
   fail. A populated commit alone cannot bypass missing evidence, and an empty
   `failures` array cannot establish zero gaps. Full archive/ZIP inventory
   comparisons remain required; successful mapping is not case execution.
2. **Schema dependencies.** Fetching and offline verification share the same
   source-derived dependency discovery. RNC `include`/inline `external`, RNG
   `include`/`externalRef`, XSD include/import/redefine/override locations, and
   literal external DTD entity declarations are registered; XML schema
   references honor inherited `xml:base`. Comments/quoted keyword text do not
   become RNC imports. Cycles terminate through the existing URL deduplication
   and archive budgets. Offline verification re-discovers actual references,
   rather than trusting a shortened dependency list. Schema includes are not
   deferred bibliography references.
3. **Fixed version identity.** A dated REC/NOTE request must resolve to the same
   fixed URI and its `This version` link must identify that URI. An incidental
   occurrence of the old URL elsewhere in the body cannot qualify a newer REC.
   The only URI spelling equivalence used is http→https for older W3C self-links
   such as XML Fifth Edition. The frozen year/path/edition are not relaxed.

`fetch --complete-schemas` is an explicit reviewed archive extension: it verifies
previous byte hashes and fixed document identities, keeps old source bytes and
download timestamps, and adds only source-discovered schema dependencies.
Ordinary locked `fetch` continues to reject drift. The old checkpoint remains
in Git history and its received files/logs remain preserved.

## Actual added source files

All three files are pinned to EPUBCheck source commit
`029831b8f477e4519e9734c984ee24357547a698`, not moving main:

| Relative schema path | Bytes | SHA-256 |
| --- | ---: | --- |
| `30/mod/datatypes.rnc` | 7966 | `4ecd6dfc146993af602145983b860dc971158aa0ddcdc7062028f385d653787c` |
| `30/package-30.rnc` | 6997 | `557d1c4443acf7f2587c11a3dfee9841470ed7c65db2b49ce4ff3d22aa0ca9f0` |
| `30/mod/epub-prefix-attr.rnc` | 455 | `d1a3a8872e7013e4f25f46d898f6b520ce4ea06156612e93036cfd726842df71` |

The prefix module is the necessary additional transitive include from
package-30.rnc. The new manifest contains **107 assets / 12,565,293 raw bytes**.
All previous 104 raw sources retain their exact bytes/hashes. This closure
check is not execution of a schema validator, validation of a compiled module
assembly, or evidence of product XML/DTD compatibility. Escaped compact-schema
URI syntax that is not handled fails explicitly for further archive review;
this utility is not a general RELAX NG or DTD implementation. T3's product
offline DTD semantics are still a separate, unimplemented requirement.

## Executed verification

```sh
python3 -m unittest discover -s scripts -p 'test_*.py' -v
python3 scripts/epub33_assets.py fetch --complete-schemas
python3 scripts/epub33_assets.py index
python3 scripts/epub33_assets.py verify
python3 scripts/epub33_tests.py verify
python3 scripts/epub33_tests.py reproduce
python3 scripts/epub33_upstreams.py verify
python3 scripts/epub33_assets.py gate
```

27 tests pass, including the three unchanged parent negatives, inline external
references, comments/quoted-keyword exclusions, inherited XML base, XSD import
without location, a bounded two-schema cycle and hiding a transitive schema by
removing both manifest entry and declared dependency. An incidental old REC URI
is rejected; an older http self-link for the exact edition is accepted.
The assets verify reports 107; official-source verify reports 169 cases / zero
source gaps / executed:false; upstream verify still reports the one deferred
license gap, not a legal or adoption PASS. The semantic gate still exits 1.

Fresh `go test -count=1 ./...`, `go test -race -count=1 ./...`, and `go vet ./...`
all exit 0; validation197.407s / race206.569s and workspace132.323s / race157.217s.
These execute the fixed 5.3.0 checker regressions again, not the 169 official
reading-system cases. No product Go code, main documents or parent acceptance
file was changed. Main workspace T1 drafts remain preserved; no push or Droid
request. Core semantic matrix remains pending. A bounded Accessibility1.1
research draft is awaiting integration review and is not counted as accepted
semantic coverage or Factory Droid evidence.
