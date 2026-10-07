#!/usr/bin/env python3
"""Deterministic derivation of text/html-dom-3.2.5.2.1.txt from the archived
original/html-dom.html. Raw assets are never modified; the derived excerpt
removes markup and trailing whitespace so the text asset is whitespace-clean.
After regenerating the excerpt every registered bytes/SHA-256 pair in
manifest.json and ATTRIBUTION.md is verified against the archived bytes, so the
metadata cannot drift from the assets."""
import hashlib
import json
import re
import pathlib
import sys

base = pathlib.Path(__file__).resolve().parent.parent
src = (base / "original" / "html-dom.html").read_text(encoding="utf-8", errors="replace")
i = src.find("id=metadata-content")
j = src.find("3.2.5.2.2", i)
chunk = re.sub(r"<[^>]+>", "", src[i:j])
k = chunk.find("Metadata content is content that sets up")
if k > 0:
    chunk = chunk[k:]
lines = [line.rstrip() for line in chunk.splitlines()]
text = "\n".join(lines).strip() + "\n"
(base / "text" / "html-dom-3.2.5.2.1.txt").write_text(
    "Extracted from the archived HTML Standard (dom.html) section 3.2.5.2.1 Metadata content.\n"
    "The raw file is preserved unmodified; this text excerpt is a derived convenience copy.\n\n" + text,
    encoding="utf-8",
)

# Verify the registered metadata of every batch asset against its actual bytes.
specs = base.parent.parent
manifest = json.loads((base / "manifest.json").read_text(encoding="utf-8"))
attribution = (base / "ATTRIBUTION.md").read_text(encoding="utf-8")
rows = {
    name: (int(size), digest)
    for name, size, digest in re.findall(
        r"^\| `([^`]+)` \|[^|]*\| (\d+) \| `([0-9a-f]{64})` \|$", attribution, re.M
    )
}
failures = []
for asset in manifest["assets"]:
    data = (specs / asset["path"]).read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if asset["bytes"] != len(data) or asset["sha256"] != digest:
        failures.append(
            "manifest %s: registered %d/%s actual %d/%s"
            % (asset["path"], asset["bytes"], asset["sha256"][:12], len(data), digest[:12])
        )
for name, (size, digest) in rows.items():
    data = (base / name).read_bytes()
    actual = hashlib.sha256(data).hexdigest()
    if size != len(data) or digest != actual:
        failures.append(
            "attribution %s: registered %d/%s actual %d/%s"
            % (name, size, digest[:12], len(data), actual[:12])
        )
missing = {a["path"].split("t2-fix-proposal/", 1)[-1] for a in manifest["assets"]} - set(rows)
if missing:
    failures.append("attribution rows missing: %s" % sorted(missing))
if failures:
    sys.exit("metadata verification failed:\n  " + "\n  ".join(failures))
print("metadata verified: %d manifest assets, %d attribution rows" % (len(manifest["assets"]), len(rows)))
