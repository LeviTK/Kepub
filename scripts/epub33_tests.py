#!/usr/bin/env python3
"""Freeze 3.3 report IDs and match official ZIP bytes to historical source.

Never executes an EPUB or the upstream generator. Source matching compares the
entire uncompressed file inventory, not only schema:version or directory names.
Generation is deterministic; website ZIP and local ZIP hashes remain distinct.
"""
import argparse
import io
import inspect
import json
from pathlib import Path, PurePosixPath
import subprocess
import sys
import tarfile
import zipfile
import zlib

from epub33_assets import DOM, ROOT, download, normalized, sha, write_json


# The report repeats this case ID, but its source instructions require two
# publications with the same identifier. A case count is not an artifact count.
PAIRED_CASES = {"pkg-unique-id": ("pkg-unique-id_duplicate",)}


def git(repo, *args):
    return subprocess.check_output(["git", "-C", str(repo), *args], stderr=subprocess.DEVNULL)


def source_files(archive):
    files = {}
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for member in tar:
            if member.isfile():
                p = PurePosixPath(member.name)
                if p.is_absolute() or ".." in p.parts or member.name in files:
                    raise ValueError("unsafe/duplicate source path")
                files[member.name] = tar.extractfile(member).read()
            elif not member.isdir() and member.name != "pax_global_header":
                raise ValueError("non-regular source entry")
    return files


def zip_files(data):
    files = {}
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        if sum(i.file_size for i in z.infolist()) > 64 << 20:
            raise ValueError("uncompressed official artifact over 64MiB")
        for info in z.infolist():
            if info.is_dir():
                continue
            p = PurePosixPath(info.filename)
            if p.is_absolute() or ".." in p.parts or info.filename in files:
                raise ValueError("unsafe/duplicate ZIP entry")
            files[info.filename] = z.read(info)
    return files


def generate(files):
    out = io.BytesIO()
    with zipfile.ZipFile(out, "w") as z:
        for name in ["mimetype"] + sorted(n for n in files if n != "mimetype"):
            i = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
            i.compress_type = zipfile.ZIP_STORED if name == "mimetype" else zipfile.ZIP_DEFLATED
            i.create_system = 0
            z.writestr(i, files[name])
    return out.getvalue()


def report_cases(root):
    dom = DOM((root / "original/test-index.html").read_text(encoding="utf-8")).root
    cases = {}
    for tr in dom.walk():
        if tr.tag != "tr":
            continue
        cells = [n for n in tr.children if hasattr(n, "tag") and n.tag == "td"]
        if not cells or not cells[0].attrs.get("id"):
            continue
        name = cells[0].attrs["id"]
        fields = {"expected": normalized(cells[1]), "normativeLevel": normalized(cells[2]),
                  "reportReferences": [n.attrs["href"] for n in cells[4].walk() if n.tag == "a" and n.attrs.get("href")]}
        if name in cases:
            if any(cases[name][k] != v for k, v in fields.items()):
                raise ValueError("inconsistent repeated report case")
            cases[name]["reportOccurrences"] += 1
        else:
            cases[name] = {**fields, "reportOccurrences": 1}
    return cases


def generator_runtime():
    return {"python": sys.version, "zlibCompile": zlib.ZLIB_VERSION,
            "zlibRuntime": zlib.ZLIB_RUNTIME_VERSION,
            "generatorFunctionSHA256": sha(inspect.getsource(generate).encode())}


def match_source(root, repo, row, artifact):
    name = row["id"]
    source_path = row["sourcePath"]
    expected = zip_files(artifact)
    history = git(repo, "log", "--all", "--format=%H", "--", source_path).decode().splitlines()
    for commit in list(dict.fromkeys([row["reportCommit"]] + history)):
        try:
            archive = git(repo, "archive", f"{commit}:{source_path}")
            files = source_files(archive)
        except (subprocess.CalledProcessError, ValueError):
            continue
        if files != expected:
            continue
        archive_path = f"official-tests/source/{name}.tar"
        generated_path = f"official-tests/generated/{name}.epub"
        generated = generate(files)
        (root / archive_path).parent.mkdir(parents=True, exist_ok=True)
        (root / generated_path).parent.mkdir(parents=True, exist_ok=True)
        (root / archive_path).write_bytes(archive)
        (root / generated_path).write_bytes(generated)
        row.update(status="exact-source-file-inventory-match; semantic-review-pending",
                   sourceCommit=commit, sourceArchivePath=archive_path, sourceArchiveSHA256=sha(archive),
                   generatedArtifactPath=generated_path, generatedSHA256=sha(generated),
                   contentInventory={n: sha(d) for n, d in sorted(files.items())},
                   comparison="all paths and uncompressed bytes equal; ZIP hashes distinct")
        return
    raise ValueError("no historical source tree matches full official ZIP inventory")


def capture_pairs(root, repo, index):
    for row in index["cases"]:
        if row.get("pairedFixtures"):
            continue  # Existing frozen evidence is verified, never redownloaded.
        pairs = []
        for name in PAIRED_CASES.get(row["id"], ()):
            url = f"https://w3c.github.io/epub-tests/tests/{name}.epub"
            artifact, final, _ = download(url)
            pair = {"id": name, "reportCommit": row["reportCommit"],
                    "sourcePath": f"tests/{name}", "websiteUrl": url,
                    "websiteFinalUrl": final, "websiteArtifactPath": f"official-tests/website/{name}.epub",
                    "websiteSHA256": sha(artifact), "sourceCommit": None,
                    "role": "Required paired publication; same identifier, different title/content",
                    "executed": False, "generatorRuntime": generator_runtime()}
            match_source(root, repo, pair, artifact)
            if pair["sourceCommit"] != row["reportCommit"]:
                raise ValueError("paired publication not found at fixed report commit")
            (root / pair["websiteArtifactPath"]).write_bytes(artifact)
            pairs.append(pair)
        if pairs:
            row["pairedFixtures"] = pairs


def resolve_missing(root, repo):
    """Resolve two actually investigated historical names, retain first failures."""
    index_path = root / "official-tests/index.json"
    index = json.loads(index_path.read_text())
    index.setdefault("initialCaptureFailures", index["failures"])
    failures = []
    for row in index["cases"]:
        if row["sourceCommit"]:
            continue
        try:
            if row["id"] == "ocf-font_obfuscation-bis":
                row["sourcePath"] = "tests/ocf-font_obfuscation_bis"
                url = "https://w3c.github.io/epub-tests/tests/ocf-font_obfuscation_bis.epub"
                artifact, final, _ = download(url)
                row.update(websiteUrl=url, websiteFinalUrl=final,
                           sourceNameReason="Report ID uses -bis; actual source dirname uses _bis; dc:identifier and complete content match")
                path = "official-tests/website/ocf-font_obfuscation-bis.epub"
                row.update(websiteArtifactPath=path, websiteSHA256=sha(artifact))
            elif row["id"] == "cnt-css-fonts":
                commit = "d22a6b70b1e221cef718ebf982d53c69d0f8442d"
                artifact = git(repo, "show", f"{commit}:tests/cnt-css-fonts.epub")
                path = "official-tests/historical/cnt-css-fonts.epub"
                row.update(historicalArtifactPath=path, historicalArtifactSHA256=sha(artifact),
                           historicalArtifactCommit=commit,
                           websiteStatus="404; original case split by 4f5a851bcf35fda29ff188795eacde4e054d0371; NOT replaced with four newer cases")
            else:
                raise ValueError("unknown unresolved source requires investigation, not a guessed alias")
            (root / path).parent.mkdir(parents=True, exist_ok=True)
            (root / path).write_bytes(artifact)
            match_source(root, repo, row, artifact)
        except (ValueError, subprocess.CalledProcessError) as exc:
            failures.append({"id": row["id"], "reason": str(exc)})
    index["failures"] = failures
    write_json(index_path, index)
    if failures:
        raise ValueError("unresolved historical source gaps remain")


def capture(root, repo):
    dest = root / "official-tests"
    if (dest / "index.json").exists():
        raise ValueError("test snapshot exists; use separate capture root for upgrade")
    commit = git(repo, "rev-parse", "HEAD").decode().strip()
    for name, path in [("test-index", "epub33/index.html"), ("test-results", "epub33/results.html")]:
        if git(repo, "show", f"{commit}:{path}") != (root / f"original/{name}.html").read_bytes():
            raise ValueError(f"report snapshot doesn't match archived website: {path}")
    dest.mkdir(parents=True, exist_ok=True)
    license_data = git(repo, "show", f"{commit}:LICENSE.md")
    (dest / "LICENSE.upstream.md").write_bytes(license_data)
    upstream_generator = git(repo, "show", f"{commit}:tests/generateEpubs.sh")
    (dest / "generateEpubs.upstream.sh").write_bytes(upstream_generator)
    dom = DOM((root / "original/test-index.html").read_text()).root
    rows, failures = [], []
    seen = {}
    for tr in dom.walk():
        if tr.tag != "tr":
            continue
        cells = [n for n in tr.children if hasattr(n, "tag") and n.tag == "td"]
        if not cells or not cells[0].attrs.get("id"):
            continue
        name = cells[0].attrs["id"]
        if name in seen:
            seen[name]["reportOccurrences"] += 1
            continue
        refs = [n.attrs["href"] for n in cells[4].walk() if n.tag == "a" and n.attrs.get("href")]
        row = {"id": name, "reportOccurrences": 1, "expected": normalized(cells[1]),
               "normativeLevel": normalized(cells[2]), "reportReferences": refs,
               "reportCommit": commit, "reportPath": "epub33/index.html",
               "reportSHA256": sha((root / "original/test-index.html").read_bytes()),
               "target": "EPUB3.3 report expectations; anchors need fixed REC clause review",
               "executed": False, "applicability": "requires clause-specific review; no conformance PASS",
               "status": "source-unresolved", "sourceCommit": None, "sourcePath": f"tests/{name}",
               "sourceArchivePath": None, "sourceArchiveSHA256": None,
               "websiteUrl": f"https://w3c.github.io/epub-tests/tests/{name}.epub",
               "websiteArtifactPath": None, "websiteSHA256": None,
               "generatedArtifactPath": None, "generatedSHA256": None,
               "generator": "scripts/epub33_tests.py:generate (deterministic ZipInfo1980 sorted files)",
               "generatorRuntime": generator_runtime()}
        seen[name] = row
        rows.append(row)
        try:
            website, final, _ = download(row["websiteUrl"])
            row["websiteFinalUrl"] = final
            zip_files(website)
            web_path = f"official-tests/website/{name}.epub"
            (root / web_path).parent.mkdir(parents=True, exist_ok=True)
            (root / web_path).write_bytes(website)
            row.update(websiteArtifactPath=web_path, websiteSHA256=sha(website))
            match_source(root, repo, row, website)
        except (ValueError, zipfile.BadZipFile, subprocess.CalledProcessError) as exc:
            failures.append({"id": name, "reason": str(exc)})
    try:
        capture_pairs(root, repo, {"cases": rows})
    except (ValueError, subprocess.CalledProcessError) as exc:
        failures.append({"id": "paired-publications", "reason": str(exc)})
    write_json(dest / "index.json", {"schemaVersion": 1, "reportCommit": commit,
                                    "reportSourcePath": "epub33/ (historically history/epub33/)",
                                    "upstreamGeneratorSHA256": sha(upstream_generator), "licenseSHA256": sha(license_data),
                                    "semanticReviewComplete": False, "cases": rows, "failures": failures})
    if failures:
        raise ValueError(f"{len(failures)} test-source gaps recorded; S0 item4 remains blocked")


def reproduce(root):
    verify(root)
    path = root / "official-tests/index.json"
    index = json.loads(path.read_text())
    runtime = generator_runtime()
    for row in index["cases"]:
        if not row["sourceCommit"]:
            raise ValueError("cannot reproduce unresolved source")
        for artifact in [row] + row.get("pairedFixtures", []):
            files = source_files((root / artifact["sourceArchivePath"]).read_bytes())
            if sha(generate(files)) != artifact["generatedSHA256"]:
                raise ValueError(f"generator output differs: {artifact['id']}")
            artifact["generatorRuntimeAtOfflineReproduction"] = runtime
    write_json(path, index)


def verify(root):
    index = json.loads((root / "official-tests/index.json").read_text())
    expected = report_cases(root)
    if len(index["cases"]) != len(expected) or {r["id"] for r in index["cases"]} != set(expected):
        raise ValueError("missing/duplicate/extra official report mapping")
    if index["failures"] or any(not r.get("sourceCommit") for r in index["cases"]):
        raise ValueError("unresolved official source gaps")
    if index.get("executed", False) is not False:
        raise ValueError("official source archive must not invent reading-system execution")
    report_hash = sha((root / "original/test-index.html").read_bytes())
    dest = root / "official-tests"
    if (sha((dest / "LICENSE.upstream.md").read_bytes()) != index["licenseSHA256"] or
            sha((dest / "generateEpubs.upstream.sh").read_bytes()) != index["upstreamGeneratorSHA256"]):
        raise ValueError("official license/generator drift")
    for row in index["cases"]:
        if row["reportSHA256"] != report_hash or any(row[k] != v for k, v in expected[row["id"]].items()):
            raise ValueError("official report provenance drift")
        pairs = row.get("pairedFixtures", [])
        if tuple(p.get("id") for p in pairs) != PAIRED_CASES.get(row["id"], ()):
            raise ValueError("missing/duplicate/extra required paired publication")
        for pair in pairs:
            if (pair.get("sourcePath") != f"tests/{pair['id']}" or
                    pair.get("sourceCommit") != row["reportCommit"]):
                raise ValueError("paired publication fixed source identity drift")
    artifacts = [artifact for row in index["cases"] for artifact in [row] + row.get("pairedFixtures", [])]
    for row in artifacts:
        required = ("sourceCommit", "sourcePath", "sourceArchivePath", "sourceArchiveSHA256", "generatedArtifactPath", "generatedSHA256")
        if any(not row.get(k) for k in required):
            raise ValueError(f"incomplete official source evidence: {row['id']}")
        executed = row.get("executed") if row["id"] in expected else row.get("executed", False)
        if executed is not False or row.get("result", "not-tested") != "not-tested":
            raise ValueError("official source archive must not invent reading-system execution")
        if row["id"] in expected and row.get("status") != "exact-source-file-inventory-match; semantic-review-pending":
            raise ValueError("official source archive status is not execution evidence")
        if not ((row.get("websiteArtifactPath") and row.get("websiteSHA256")) or
                (row.get("historicalArtifactPath") and row.get("historicalArtifactSHA256"))):
            raise ValueError(f"missing official original artifact: {row['id']}")
        for path_key, hash_key in (("websiteArtifactPath", "websiteSHA256"), ("historicalArtifactPath", "historicalArtifactSHA256")):
            if bool(row.get(path_key)) != bool(row.get(hash_key)):
                raise ValueError(f"incomplete official artifact evidence: {row['id']}")
        for prefix in ("websiteArtifact", "sourceArchive", "generatedArtifact"):
            path_key = prefix + "Path"
            hash_key = {"websiteArtifact": "websiteSHA256", "sourceArchive": "sourceArchiveSHA256", "generatedArtifact": "generatedSHA256"}[prefix]
            if row[path_key] and sha((root / row[path_key]).read_bytes()) != row[hash_key]:
                raise ValueError(f"official test hash drift: {row['id']}")
        if row["sourceCommit"]:
            files = source_files((root / row["sourceArchivePath"]).read_bytes())
            artifact_path = row["websiteArtifactPath"] or row.get("historicalArtifactPath")
            if row.get("historicalArtifactPath") and sha((root / artifact_path).read_bytes()) != row["historicalArtifactSHA256"]:
                raise ValueError(f"historical artifact drift: {row['id']}")
            if files != zip_files((root / artifact_path).read_bytes()) or files != zip_files((root / row["generatedArtifactPath"]).read_bytes()):
                raise ValueError(f"official test inventory drift: {row['id']}")
            if {n: sha(d) for n, d in sorted(files.items())} != row["contentInventory"]:
                raise ValueError(f"official content hash mismatch: {row['id']}")
    return {"cases": len(index["cases"]), "reportRows": sum(r["reportOccurrences"] for r in index["cases"]),
            "publications": len(artifacts), "sourceGaps": len(index["failures"]),
            "semanticReviewComplete": index["semanticReviewComplete"],
            "executed": any(row["executed"] for row in index["cases"])}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("command", choices=("capture", "resolve-missing", "capture-pairs", "reproduce", "verify"))
    p.add_argument("--root", type=Path, default=ROOT)
    p.add_argument("--source-repo", type=Path)
    args = p.parse_args()
    try:
        if args.command in ("capture", "resolve-missing", "capture-pairs"):
            if not args.source_repo:
                raise ValueError("--source-repo requires a full-history w3c/epub-tests clone")
            if args.command == "capture":
                capture(args.root, args.source_repo)
            elif args.command == "capture-pairs":
                path = args.root / "official-tests/index.json"
                index = json.loads(path.read_text())
                capture_pairs(args.root, args.source_repo, index)
                write_json(path, index)
                print(json.dumps(verify(args.root), sort_keys=True))
            else:
                resolve_missing(args.root, args.source_repo)
        elif args.command == "reproduce":
            reproduce(args.root)
        else:
            print(json.dumps(verify(args.root), sort_keys=True))
    except (ValueError, OSError, KeyError) as exc:
        p.exit(1, f"ERROR: {exc}\n")


if __name__ == "__main__":
    main()
