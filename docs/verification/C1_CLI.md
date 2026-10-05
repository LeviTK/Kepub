# C1 CLI discovery and local diagnostics

Date: 2026-10-05. Scope: CLI contract §2.1 and development plan §11.3–11.6.
Implementation and fixes were performed in this dedicated CLI workstream; no
additional thread, model task, GUI, SDK, Cobra or production dependency was added.

## Baseline and delivery

The supplied `kepub-c0.bundle` was inspected with `sha256sum` and `git bundle
verify`, imported using `git fetch kepub-c0.bundle main`, and the `c1-cli` branch
was created at the exact parent local baseline
`fdb3a5b1ec10a1b479f0e9f08f11c35ab507364f`. The transport file was deleted.
The input bundle's supplied SHA-256 was
`24c10db014ac2db242894a2b6509269032ed3238e99daab4a22d495e94eaf075`.
This baseline includes the unpublished metadata/review/export loop; implementing
directly from `origin/main` would not have included it.

Initial code commit: `a369c2e6f1f2841b3b222450020b5a4ee6d79bba`. The ordinary/race
suite, vet, installation and 18-call binary smoke below were run after that commit.
The parent's independent check then found the missing-section inspect-help filter
gap and timeout schema bound described below. Commit
`c15858a57a168e2696255d190170156237bbadbb` fixes the help gap; final code commit
`900ce1cc7c7bef973578524520037e2ce99fb457` declares the existing timeout maximum.
Targeted ordinary/race and rebuilt binary checks passed after both corrections.
The whole suite was not rerun after these fixes, per the parent's instruction;
parent combination testing remains
the final integration gate. This report is a separate documentation-only commit.
Nothing was pushed/published. App/CLI ownership is ready for explicit C2 handoff.

Changed ownership: `cmd/kepub`, `internal/app`; only the existing process runner's
output-limit parameter plus readiness probe/tests in `internal/validation` were
added. No publication/workspace implementation, root dependency, setup, README,
development plan or CLI contract was changed.

## Implemented behavior

- Command syntax is derived from the existing capability schemas. The shared
  descriptions include positional target, required/options schema, operation ID,
  status, risk, reason and result schema. They generate overview, command and
  group `--help`, and additive `commandSchemas` inside the existing capabilities
  array. Existing operation IDs and the `metadata.set` v1 schema remain intact.
- CLI validates known command/subcommand, command-specific options, required
  inputs and inspect filters before entering a file/workspace use case. Help
  skips required targets but never skips unknown/disallowed options or malformed
  supplied values. Canonical rootfile syntax is checked before book I/O.
  Duplicate options/aliases are rejected. `--`, `-o`, `-h`, `--json` and
  `--no-input` retain their roles. No interaction or implicit target was added.
- Planned commands are discoverable, not executable (exit 3 for the registered
  planned command with no arguments). Their future argument schemas remain
  unfrozen; undocumented arguments are rejected, not interpreted as executable
  functionality. `metadata.set` remains an operation submitted through plan,
  not a new direct mutation command.
- `version` uses `debug.ReadBuildInfo` and Go runtime information: module/build
  version, development flag, Go version, OS/architecture and available VCS
  revision/time/dirty metadata. It never runs Git or network queries. Go 1.27's
  untagged VCS pseudo-version and dirty builds are explicitly development builds.
- `doctor` reports core readiness, Java 17+ version evidence, complete pinned
  EPUBCheck JAR-set integrity and exact 5.3.0 version, plus optional Amp PATH
  discovery. Amp is **not executed**, including version/login. Missing tools are
  successful diagnostic collection with `formalValidationAvailable:false`, not
  conformance pass. Probe execution failures return exit 5; cancellation returns
  130. No package installation, authentication, model execution or environment
  dump occurs. Raw external output and private dependency paths are not emitted.
- Doctor uses a shared 10-second deadline for the external Java/checker probes,
  64 KiB per stdout/stderr stream, the existing process-group kill/reclamation
  and one-second pipe WaitDelay. Cleanup may additionally take up to two seconds.
  Java option/classpath injection variables are removed as in formal checking.
  The checker fingerprint is reused and verified again after version probing;
  there is no second pin. Formal checks retain their existing 16 MiB limit and
  validation semantics.
- Existing envelope, exit mapping, plan/apply/review_required, task provenance,
  accepted-only export and missing-checker no-fallback semantics are unchanged.

## Build and install

From a checked-out repository with Go 1.27.1:

```sh
mkdir -p bin
go build -o bin/kepub ./cmd/kepub
bin/kepub version --json
bin/kepub --help --json
bin/kepub task accept --help --json

# Install the independent headless binary to an explicit directory.
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install ./cmd/kepub
"$HOME/.local/bin/kepub" version --json
"$HOME/.local/bin/kepub" doctor --json
```

The installed binary does not need Go, Git, Node, Amp or a GUI to execute
capabilities/info/toc/inspect/unpack. A runtime Git checkout is not necessary.
Formal validation/pack/accept/export require Java 17+ and the **complete official
EPUBCheck 5.3.0 distribution**, not just a copied main JAR:

```sh
export KEPUB_EPUBCHECK_JAR='/absolute/path/epubcheck-5.3.0/epubcheck.jar'
kepub doctor --json
kepub validate '/absolute/path/book.epub' --json
```

Install those external dependencies separately; doctor never installs them.
An absent/mismatched checker prevents formal checking; only an explicit draft
output request permits the existing unverified draft mode. Local builds without
a release tag are development builds, not a release/installable Mac product.

## Exact verification evidence

Environment: Linux amd64 orb; `bash .agents/setup` was executed only to prepare
this development environment, without editing setup. A clean `bash -lc` reported
`go version go1.27.1 linux/amd64`, OpenJDK `17.0.20.1`, and had the managed official
EPUBCheck 5.3.0 configured. All following Go commands used that clean login
environment, so relevant real checker tests ran instead of dependency skips.

| Command | Decisive result |
| --- | --- |
| `go test -count=1 ./cmd/kepub -run 'Test(CommandOption\|Help\|Version)'` | `ok .../cmd/kepub 0.113s` |
| `go test -race -count=1 ./cmd/kepub -run 'Test(CommandOption\|Help\|Version)'` | `ok .../cmd/kepub 1.288s` |
| `go test -count=1 ./internal/validation -run TestProbe` | `ok .../internal/validation 1.934s`; pinned checker test included |
| `go test -count=1 ./...` after code commit | All packages passed; CLI `174.556s`, validation `264.730s`, workspace `200.310s` |
| `go test -race -count=1 ./...` after code commit | All packages passed; CLI `184.084s`, validation `260.241s`, workspace `212.828s`; no race report |
| `go vet ./...` after code commit | Exit 0, no output |
| `git diff --check` | Exit 0, no whitespace errors |
| `go build -o /tmp/kepub-c1 ./cmd/kepub` | Exit 0; executable used below |
| `GOBIN=/tmp/c1-installed/bin go install ./cmd/kepub` | Exit 0; installed executable independently invoked |
| `env -i PATH=/nonexistent /tmp/c1-installed/bin/kepub version --json` | Exit 0; single envelope; Go `go1.27.1`, linux/amd64, `development:true`, `modified:false`, exact code revision |
| `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/kepub-c1-darwin ./cmd/kepub` | Exit 0; `file`: `Mach-O 64-bit arm64 executable` — **cross-compilation only** |

New test evidence:

- `TestCommandOptionMatrix`: 21 command descriptions × 11 known options, with
  an independently declared allow-list; each legal option is advertised and
  parseable, and each illegal pairing fails. Each command's help is also invoked.
- `TestHelpAndCapabilityDescriptionsMatch`: exact overview/per-command help and
  capability command-schema equality in both directions.
- `TestHelpDoesNotHideInvalidInput`: unknown command/subcommand, wrong option,
  invalid inspect section, invalid timeout/rootfile and duplicate options; errors
  with nonexistent input still exit 2, and registered planned commands exit 3.
- `TestVersionAndDoctorWithoutRuntimeDependencies`: isolated PATH, unavailable
  checker, optional fake Amp discovery with an execution sentinel, and secret/
  private-path output exclusion. Amp sentinel remains absent.
- `TestProbeVersionsIntegrityAndFailure`: Java 11/21, unknown version, nonzero
  exit, output flood, short inherited deadline, corrupt checker rejected before
  execution and cancellation. `TestProbeRealPinnedChecker` verifies the real
  complete release. `TestProbeReclaimsClosedPipeDescendant` rejects residual
  writer success and checks that its delayed write never happens. Existing formal
  timeout/group reclamation/report/hash-drift tests also remain passing.
- Existing CLI/workspace suites reran real metadata planning/apply/diff, accept,
  reject, accepted export, no-op, drift, source preservation, output boundaries
  and commit/cancellation behavior; this batch did not modify those policies.

### Parent-found help-filter regression and final fix

The parent independently found that `inspect --help --direction sideways --json`
and `inspect --help --resource ../bad --json` returned exit 0 at the initial code
commit. This thread reproduced both with the built binary and first added a
failing test (`TestHelpDoesNotHideInvalidInput`: exit 0, expected 2). Validation
previously called `ValidateInspect` only when section was supplied or help was
absent. It now also calls it for any explicit resource/direction value, requiring
the references section even in help mode. Nothing else in the code was changed.

The new cases include valid `incoming` / canonical resource **without** section,
not just invalid direction/path values, so they enforce the dependency itself.
Bare `inspect --help --json` explicitly remains successful. The existing matrix
continues testing valid filters with section/resource supplied.

Post-fix targeted evidence (clean login):

```sh
go test -count=1 ./cmd/kepub -run 'Test(CommandOption|Help|Version|JSONSuccessFailureAndSelection)'
# ok .../cmd/kepub 0.115s
go test -race -count=1 ./cmd/kepub -run 'Test(CommandOption|Help|Version|JSONSuccessFailureAndSelection)'
# ok .../cmd/kepub 1.345s
go build -o /tmp/kepub-c1 ./cmd/kepub
```

Rebuilt binary: both parent-provided invocations now exit 2 with one envelope,
`ok:false`, `data:null`, `error.code:INVALID_ARGUMENT`; plain inspect help exits
0. `git diff --check` also passed. Earlier full-suite results above describe the
initial code commit, not a claimed post-fix whole-suite run.

### Parent-found timeout schema boundary

The parser already accepts only uint32 positive seconds, but all four advertised
timeout schemas previously omitted the maximum. `TestTimeoutSchemaUint32Boundary`
was first run failing on the missing source maximum. The source schemas now state
`minimum:1, maximum:4294967295`; parsing/execution policy was not changed. The test
checks source and inherited commandSchemas for publication.validate,
publication.pack, task.accept and workspace.export, accepts 4294967295 and verifies
its exact parsed duration, then rejects 4294967296 for each command.

Final targeted commands after both corrections:

```sh
go test -count=1 ./cmd/kepub -run 'Test(CommandOption|Help|Version|Timeout|JSONSuccessFailureAndSelection)'
# ok .../cmd/kepub 0.151s
go test -race -count=1 ./cmd/kepub -run 'Test(CommandOption|Help|Version|Timeout|JSONSuccessFailureAndSelection)'
# ok .../cmd/kepub 1.379s
go build -o /tmp/kepub-c1 ./cmd/kepub
```

Rebuilt `validate --help --timeout 4294967295 --json` exited 0 and advertised the
same maximum; 4294967296 exited 2 / `INVALID_ARGUMENT`. Parent also independently
reported 15 JSON calls, no-PATH reads/version/doctor, real checker readiness and
unchanged original hash; that is parent evidence, separate from this thread's
executed checks. No additional full-suite run was claimed for the schema-only fix.

### Real binary smoke

An ephemeral Python fixture driver was run twice (including after the code
commit), with the production binary and a synthesized compliant EPUB3. Its PATH
contained **only Java**, with no Amp/Node/GUI/model configuration. The fixture
contained Chinese, entity text and CRLF, and its filename was `-中文 空格.epub`.
The driver and fixtures were removed after validation; no scratch helper is a
runtime dependency.

| Invocations | Result independently asserted |
| --- | --- |
| version; ready doctor; doctor with no Java/checker; capabilities | Exit 0; development build, real checker ready versus unavailable, Amp absent, array shape and metadata v1 retained |
| info help; workspace group help | Exit 0; exactly the requested command/group descriptions |
| unknown help; unknown workspace subcommand help; info wrong section option; inspect invalid section | Exit 2 / `INVALID_ARGUMENT`, before nonexistent-input I/O |
| workspace list | Exit 3 / `CAPABILITY_UNAVAILABLE` |
| info; toc; unpack `-o`; `--json info -- -中文 空格.epub` | Exit 0; safe Chinese/space/dash target handling |
| validate with real checker; validate without Java | Pass versus exit 3 / `DEPENDENCY_UNAVAILABLE` |
| doctor against sleeping fake Java, then SIGINT | Exit 130 / `CANCELLED`; single envelope and empty stderr |

Driver result: `PASS: 18 real binary invocations`. Every parsed result was one
schemaVersion-1 envelope with matching `ok`, nonempty requestId and no stderr.
The final checker report was asserted `status:pass`. Each unpacked resource was
compared to independently supplied exact UTF-8 bytes, and the original archive's
SHA-256 was unchanged. The installed version output's revision matched the code
commit and it correctly reported the clean untagged pseudo-version as development.

## Limits and handoff

No actual Amp login/model, Calibre, GUI or user publication was used. Amp's
discovered status does not prove authentication/version/model availability.
Doctor readiness does not validate any book or prove rendering/accessibility.
Process cleanup covers the managed PGID only; it does not promise OS sandboxing
or reclamation of intentionally escaped process groups.

Darwin was cross-compiled, not executed. Apple Silicon install/runtime/release
verification remains unperformed. No release, push or deployment occurred.
C2 can now extend the existing capability schema and parser value storage, then
add its application dispatch and option-matrix expectations; do not create a
parallel capability registry or bypass the accepted snapshot/locking use case.
