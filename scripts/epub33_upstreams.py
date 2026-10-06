#!/usr/bin/env python3
"""Capture public upstream identities/licenses for research, not adoption.

Requires gh for explicit public API reads; never reads or prints its credentials.
The frozen index is checked offline by --verify. New versions go in a separate
capture directory; this command does not overwrite an existing snapshot.
"""
import argparse
import base64
import datetime
import hashlib
import json
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1] / "docs/specs/epub-3.3/upstreams"
PROJECTS = [
    ("readium/cli", "1", "T1–T4 / S2", "metadata/reading-order reference; no Workspace replacement"),
    ("readium/go-toolkit", "1", "T1–T4 / S2", "publication and resource-model reference"),
    ("veripublica/epubsana", "1", "T2", "FixProposal research; external repair adapter deferred"),
    ("VirInvictus/bindery-cli", "1", "T1 / T2", "declaration/entity/NCX counterexamples, not blanket repair"),
    ("veripublica/epubveri", "1", "T2 / S4", "optional differential-check prototype; not formal gate"),
    ("w3c/epubcheck", "2", "existing formal gate / S0 item5", "EPUBCheck5.3.0; rule coverage separately evidenced"),
    ("kovidgoyal/calibre", "2", "deferred adapter", "behavioral reference; no font embedding/download/subsetting"),
    ("daisy/ace", "3", "S4", "optional automated accessibility evidence, not certification"),
    ("w3c/epub-tests", "3", "S0 / T1–T5 / S2–S4", "3.3 source/artifact mapping separate from moving HEAD"),
    ("readium/css", "3", "S2 / S3", "reading CSS reference, not T5 tokenizer"),
    ("raitucarp/epub", "4", "T1–T4", "Reader/Writer/Editor comparison; round-trip fidelity untested"),
    ("9beach/epub-merge", "5", "deferred", "merge workflow reference, not safe title-based splitting"),
    ("jgm/pandoc", "5", "deferred", "conversion/template reference, not EPUB-preserving save"),
    ("daisy/pipeline-cli-go", "5", "deferred", "workflow client reference, not standalone conversion core"),
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def api(path):
    return subprocess.check_output(["gh", "api", path])


def capture(root):
    if (root / "index.json").exists():
        raise ValueError("snapshot exists: choose a separate --root for reviewed upgrades")
    root.mkdir(parents=True, exist_ok=True)
    records, failures = [], []
    for repo, group, phase, role in PROJECTS:
        directory = root / repo.replace("/", "--")
        directory.mkdir(exist_ok=True)
        try:
            commit = "029831b8f477e4519e9734c984ee24357547a698" if repo == "w3c/epubcheck" else "HEAD"
            raw = api(f"repos/{repo}/commits/{commit}")
            source = json.loads(raw)
            commit = source["sha"]
            (directory / "source.json").write_bytes(raw)
            record = {"repo": repo, "group": group, "commit": commit, "phase": phase, "role": role,
                      "sourceUrl": f"https://github.com/{repo}/tree/{commit}",
                      "acquiredAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      "sourceMetadataPath": str(directory.relative_to(root) / "source.json"),
                      "sourceMetadataSHA256": sha(raw), "evidenceStrength": "public-source-and-license-only",
                      "behaviorTested": False, "adoption": "research/deferred (not bundled)",
                      "licenseStatus": "not-reviewed", "licenseSPDX": None, "licensePath": None,
                      "licenseSHA256": None, "legalReview": "required before reuse/linking/distribution/service",
                      "protocol": "not-frozen; no optional adapter enabled", "dependencies": "not-installed/not-audited"}
            if repo == "w3c/epubcheck":
                record["adoption"] = "existing separately configured formal checker 5.3.0; no new adoption"
            try:
                license_raw = api(f"repos/{repo}/license?ref={commit}")
                license_data = json.loads(license_raw)
                content = base64.b64decode(license_data["content"])
                (directory / "license-api.json").write_bytes(license_raw)
                (directory / "LICENSE.upstream").write_bytes(content)
                record.update(licenseStatus="actual-file-archived; legal obligations not adjudicated",
                              licenseSPDX=license_data["license"]["spdx_id"],
                              licensePath=str(directory.relative_to(root) / "LICENSE.upstream"),
                              licenseSHA256=sha(content), licenseSourcePath=license_data["path"],
                              licenseMetadataPath=str(directory.relative_to(root) / "license-api.json"),
                              licenseMetadataSHA256=sha(license_raw))
            except subprocess.CalledProcessError:
                record["licenseStatus"] = "API license unavailable; adoption blocked"
                failures.append({"repo": repo, "kind": "license-unavailable"})
            records.append(record)
        except (subprocess.CalledProcessError, ValueError, KeyError) as exc:
            failures.append({"repo": repo, "kind": "source-unavailable", "reason": str(exc)})
    (root / "index.json").write_text(json.dumps({"schemaVersion": 1, "projects": records, "failures": failures}, ensure_ascii=False, indent=2) + "\n")
    if failures:
        raise ValueError(f"{len(failures)} upstream gaps recorded; not falsely adopted")


def verify(root):
    index = json.loads((root / "index.json").read_text())
    if ({p[0] for p in PROJECTS} != {r["repo"] for r in index["projects"]} or
            len(index["projects"]) != len(PROJECTS)):
        raise ValueError("missing/extra upstream role")
    expected_failures = []
    for r in index["projects"]:
        adoption = ("existing separately configured formal checker 5.3.0; no new adoption"
                    if r["repo"] == "w3c/epubcheck" else "research/deferred (not bundled)")
        if (r.get("behaviorTested") is not False or r.get("adoption") != adoption or
                r.get("evidenceStrength") != "public-source-and-license-only"):
            raise ValueError(f"upstream research must not invent behavior/adoption: {r['repo']}")
        for key in ("sourceMetadata", "license", "licenseMetadata"):
            if r.get(key + "Path"):
                path = Path(r[key + "Path"])
                if path.is_absolute() or ".." in path.parts or sha((root / path).read_bytes()) != r[key + "SHA256"]:
                    raise ValueError(f"upstream hash/path mismatch: {r['repo']}")
        metadata = json.loads((root / r["sourceMetadataPath"]).read_text())
        if r["commit"] != metadata["sha"]:
            raise ValueError(f"upstream commit mismatch: {r['repo']}")
        directory = root / r["repo"].replace("/", "--")
        if not r.get("licensePath"):
            if (r.get("licenseSHA256") is not None or r.get("licenseSPDX") is not None or
                    r.get("licenseMetadataPath") or (directory / "LICENSE.upstream").exists() or
                    r.get("licenseStatus") != "API license unavailable; adoption blocked"):
                raise ValueError(f"upstream missing-license evidence mismatch: {r['repo']}")
            expected_failures.append({"repo": r["repo"], "kind": "license-unavailable"})
        else:
            license_data = json.loads((root / r["licenseMetadataPath"]).read_text())
            if (base64.b64decode(license_data["content"]) != (root / r["licensePath"]).read_bytes() or
                    r["licenseSPDX"] != license_data["license"]["spdx_id"] or
                    r["licenseSourcePath"] != license_data["path"] or
                    r["licenseStatus"] != "actual-file-archived; legal obligations not adjudicated"):
                raise ValueError(f"upstream license metadata mismatch: {r['repo']}")
    if index["failures"] != expected_failures:
        raise ValueError("upstream license gaps differ from required evidence")
    return {"projects": len(index["projects"]), "gaps": len(expected_failures),
            "behaviorTested": any(r["behaviorTested"] for r in index["projects"])}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("command", choices=("capture", "verify"))
    p.add_argument("--root", type=Path, default=ROOT)
    args = p.parse_args()
    try:
        if args.command == "capture":
            capture(args.root)
        else:
            print(json.dumps(verify(args.root), sort_keys=True))
    except (ValueError, OSError, KeyError) as exc:
        p.exit(1, f"ERROR: {exc}\n")


if __name__ == "__main__":
    main()
