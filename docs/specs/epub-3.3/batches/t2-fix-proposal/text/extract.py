#!/usr/bin/env python3
"""Deterministic derivation of text/html-dom-3.2.5.2.1.txt from the archived
original/html-dom.html. Raw assets are never modified; the derived excerpt
removes markup and trailing whitespace so the text asset is whitespace-clean."""
import re
import pathlib

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
