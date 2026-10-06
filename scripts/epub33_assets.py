#!/usr/bin/env python3
"""Bounded, immutable EPUB 3.3 source archive and offline inventory.

Python standard library plus curl. No document scripts are evaluated. Fetching
is explicit; verify and index are entirely offline. Mechanical inventory is not
a semantic review, conformance result, or the S0 completion gate.
"""
import argparse
import collections
import datetime
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import urldefrag, urljoin, urlparse
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[1] / "docs/specs/epub-3.3"
SEEDS = [
    ("epub", "normative", "REC", "https://www.w3.org/TR/2026/REC-epub-33-20260113/"),
    ("rs", "normative", "REC", "https://www.w3.org/TR/2024/REC-epub-rs-33-20241017/"),
    ("a11y", "normative", "REC", "https://www.w3.org/TR/2024/REC-epub-a11y-11-20241017/"),
    ("css", "supporting-note", "NOTE", "https://www.w3.org/TR/2026/NOTE-css-2026-20260622/"),
    ("overview", "supporting-note", "NOTE", "https://www.w3.org/TR/2025/NOTE-epub-overview-33-20250313/"),
    ("techniques", "supporting-note", "NOTE", "https://www.w3.org/TR/2025/NOTE-epub-a11y-tech-11-20250313/"),
    ("ssv", "supporting-note", "NOTE", "https://www.w3.org/TR/2026/NOTE-epub-ssv-11-20260528/"),
    ("aria", "supporting-note", "NOTE", "https://www.w3.org/TR/2023/NOTE-epub-aria-authoring-11-20230314/"),
    ("multi-rend", "supporting-note", "NOTE", "https://www.w3.org/TR/2026/NOTE-epub-multi-rend-11-20260120/"),
    ("tts", "supporting-note", "NOTE", "https://www.w3.org/TR/2025/NOTE-epub-tts-10-20250828/"),
    ("eaa", "supporting-note", "NOTE", "https://www.w3.org/TR/2025/NOTE-epub-a11y-eaa-mapping-20250828/"),
    ("xml", "external-dependency", "REC", "https://www.w3.org/TR/2008/REC-xml-20081126/"),
    ("namespaces", "external-dependency", "REC", "https://www.w3.org/TR/2009/REC-xml-names-20091208/"),
    ("html-xml", "external-dependency", "Living Standard snapshot", "https://html.spec.whatwg.org/multipage/xhtml.html"),
    ("errata", "errata", "live snapshot", "https://w3c.github.io/epub-specs/epub33/errata.html"),
    ("implementation-report", "test", "3.3 report snapshot", "https://w3c.github.io/epub-specs/epub33/reports/"),
    ("test-index", "test", "3.3 report snapshot", "https://w3c.github.io/epub-tests/epub33/"),
    ("test-results", "test", "3.3 report snapshot", "https://w3c.github.io/epub-tests/epub33/results.html"),
    ("errata-confirmed", "errata", "GitHub issues snapshot", "https://api.github.com/repos/w3c/epub-specs/issues?state=all&labels=Errata&per_page=100&page=1"),
    ("errata-raised", "errata", "GitHub issues snapshot", "https://api.github.com/repos/w3c/epub-specs/issues?state=all&labels=ErratumRaised&per_page=100&page=1"),
]
VOID = set("area base br col embed hr img input link meta param source track wbr".split())
KEYWORDS = re.compile(r"\b(?:MUST NOT|SHALL NOT|SHOULD NOT|NOT RECOMMENDED|MUST|SHALL|SHOULD|REQUIRED|RECOMMENDED|MAY|OPTIONAL)\b")
EPUBCHECK_SCHEMA_COMMIT = "029831b8f477e4519e9734c984ee24357547a698"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


class Node:
    def __init__(self, tag, attrs=(), parent=None):
        self.tag, self.attrs, self.parent = tag, dict(attrs), parent
        self.children = []

    def walk(self):
        yield self
        for child in self.children:
            if isinstance(child, Node):
                yield from child.walk()

    def text(self):
        if self.tag in ("script", "style"):
            return ""
        return "".join(c.text() if isinstance(c, Node) else c for c in self.children)

    def ancestors(self):
        n = self
        while n:
            yield n
            n = n.parent

    def path(self):
        bits = []
        for n in self.ancestors():
            if n.parent:
                peers = [x for x in n.parent.children if isinstance(x, Node) and x.tag == n.tag]
                bits.append(f"{n.tag}[{peers.index(n) + 1}]")
        return "/" + "/".join(reversed(bits))

    def section(self):
        for n in self.ancestors():
            if n.tag in ("section", "div") and n.attrs.get("id"):
                return n.attrs["id"]
        return "document"


class DOM(HTMLParser):
    def __init__(self, data):
        super().__init__(convert_charrefs=True)
        self.root = Node("document")
        self.stack = [self.root]
        self.feed(data)
        self.close()

    def handle_starttag(self, tag, attrs):
        # Published HTML (notably Bikeshed CSS) uses optional end tags. Only
        # inventory structure is built here; never use this for EPUB editing.
        closes = {"li": ({"li"}, {"ul", "ol"}),
                  "dt": ({"dt", "dd"}, {"dl"}), "dd": ({"dt", "dd"}, {"dl"}),
                  "td": ({"td", "th"}, {"tr"}), "th": ({"td", "th"}, {"tr"}),
                  "tr": ({"tr"}, {"table", "tbody", "thead", "tfoot"})}
        if tag in closes:
            targets, barriers = closes[tag]
            for i in range(len(self.stack) - 1, 0, -1):
                if self.stack[i].tag in targets:
                    self.stack = self.stack[:i]
                    break
                if self.stack[i].tag in barriers:
                    break
        if tag in {"p", "div", "section", "table", "ul", "ol", "dl", "h1", "h2", "h3", "h4", "pre"}:
            for i in range(len(self.stack) - 1, 0, -1):
                if self.stack[i].tag == "p":
                    self.stack = self.stack[:i]
                    break
                if self.stack[i].tag in {"div", "section", "td", "dd", "li", "body"}:
                    break
        node = Node(tag, attrs, self.stack[-1])
        self.stack[-1].children.append(node)
        if tag not in VOID:
            self.stack.append(node)

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in VOID:
            self.stack.pop()

    def handle_endtag(self, tag):
        for i in range(len(self.stack) - 1, 0, -1):
            if self.stack[i].tag == tag:
                self.stack = self.stack[:i]
                break

    def handle_data(self, data):
        self.stack[-1].children.append(data)


def normalized(node):
    return " ".join(node.text().split())


def html_ok(data, url, minimum=300, requested_url=None):
    text = data.decode("utf-8")
    root = DOM(text).root
    visible = normalized(root)
    includes = any(n.attrs.get("data-include") for n in root.walk())
    # A shell with explicit included body is accepted only provisionally:
    # verify() requires the substantive included files, not only this HTML.
    if not includes and (len(visible) < minimum or not any(n.tag in ("h1", "h2", "table") for n in root.walk())):
        raise ValueError(f"empty/dynamic shell or non-document: {url}")
    if any(s in visible.lower() for s in ("verify you are human", "just a moment...", "access denied")):
        raise ValueError(f"challenge/error page: {url}")
    expected = requested_url or url
    if re.fullmatch(r"https?://[^/]+/(?:TR/)?\d{4}/(?:REC|NOTE)-[^/]+/", expected):
        # Older W3C RECs publish http self-links but are served over https.
        canonical = lambda value: re.sub(r"^http:", "https:", urldefrag(value)[0])
        versions = []
        for node in root.walk():
            if node.tag != "dt" or normalized(node).rstrip(":").lower() != "this version":
                continue
            siblings = node.parent.children[node.parent.children.index(node) + 1:]
            dd = next((n for n in siblings if isinstance(n, Node)), None)
            if dd and dd.tag == "dd":
                versions += [canonical(urljoin(url, n.attrs["href"])) for n in dd.walk()
                             if n.tag == "a" and n.attrs.get("href")]
        if canonical(url) != canonical(expected) or set(versions) != {canonical(expected)}:
            raise ValueError(f"fixed version identity missing/mismatched: {expected}")
    return root


def resources(root, base):
    """Display assets, same-document pages, schemas; not arbitrary Web links."""
    found = set()
    for n in root.walk():
        if n.attrs.get("data-include"):
            found.add(urljoin(base, n.attrs["data-include"]))
        raw = None
        if n.tag in ("img", "script", "source"):
            raw = n.attrs.get("src")
        elif n.tag == "object":
            raw = n.attrs.get("data")
        elif n.tag == "link" and "stylesheet" in n.attrs.get("rel", "").split():
            raw = n.attrs.get("href")
        elif n.tag == "a":
            href = n.attrs.get("href", "")
            target = urldefrag(urljoin(base, href))[0]
            if not href.startswith("#") and (re.search(r"\.(?:rng|rnc|xsd|dtd|ent)(?:$|\?)", target) or
                    (target.startswith(base) and target != base and urlparse(target).path.endswith(".html")) or
                    (base.startswith("https://w3c.github.io/epub-specs/epub33/reports/") and
                     target.startswith("https://w3c.github.io/epub-specs/epub33/reports/") and target.endswith(".html"))):
                raw = href
        if raw:
            target = urldefrag(urljoin(base, raw))[0]
            schema = re.fullmatch(r"https://github.com/w3c/epubcheck/blob/main/(.+)", target)
            if schema:
                target = f"https://raw.githubusercontent.com/w3c/epubcheck/{EPUBCHECK_SCHEMA_COMMIT}/{schema.group(1)}"
            if urlparse(target).scheme in ("http", "https") and target != base:
                found.add(target)
    return sorted(found)


def download(url):
    with tempfile.TemporaryDirectory() as tmp:
        file = Path(tmp) / "body"
        run = subprocess.run(["curl", "--fail", "--silent", "--show-error", "--location",
                              "--max-time", "90", "--proto", "=https,http", "--proto-redir", "=https,http",
                              "--max-filesize", str(32 << 20), "--output", str(file),
                              "--write-out", "%{url_effective}\n%{content_type}", url],
                             capture_output=True, text=True)
        if run.returncode:
            raise ValueError(f"download failed {url}: {run.stderr.strip()}")
        final, content_type = run.stdout.split("\n", 1)
        return file.read_bytes(), final, content_type


def schema_dependencies(data, base):
    suffix = Path(urlparse(base).path).suffix.lower()
    relative = []
    if suffix == ".rnc":
        tokens = re.findall(r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|#[^\n]*|\b(?:include|external)\b|\S', data.decode("utf-8"))
        tokens = [t for t in tokens if not t.startswith("#")]
        for i, token in enumerate(tokens[:-1]):
            if token in ("include", "external") and tokens[i + 1][:1] in ("'", '"'):
                literal = tokens[i + 1][1:-1]
                if "\\" in literal:
                    raise ValueError("escaped schema URI needs explicit closure review")
                relative.append((base, literal))
    elif suffix in (".rng", ".xsd"):
        try:
            document = ET.fromstring(data)
        except ET.ParseError as exc:
            raise ValueError(f"invalid schema XML: {base}") from exc
        def walk(node, inherited):
            context = urljoin(inherited, node.get("{http://www.w3.org/XML/1998/namespace}base", ""))
            if node.tag in ("{http://relaxng.org/ns/structure/1.0}include", "{http://relaxng.org/ns/structure/1.0}externalRef"):
                if not node.get("href"):
                    raise ValueError("schema reference missing href")
                relative.append((context, node.get("href")))
            elif node.tag in {"{http://www.w3.org/2001/XMLSchema}" + tag for tag in ("include", "import", "redefine", "override")}:
                if node.get("schemaLocation"):
                    relative.append((context, node.get("schemaLocation")))
                elif node.tag != "{http://www.w3.org/2001/XMLSchema}import":
                    raise ValueError("schema reference missing schemaLocation")
            for child in node:
                walk(child, context)
        walk(document, base)
    elif suffix in (".dtd", ".ent"):
        text = re.sub(r"<!--.*?-->", "", data.decode("utf-8"), flags=re.S)
        pattern = r'<!ENTITY\s+(?:%\s+)?[^\s]+\s+(?:SYSTEM\s+["\']([^"\']+)["\']|PUBLIC\s+["\'][^"\']*["\']\s+["\']([^"\']+)["\'])'
        relative += [(base, system or public_system) for system, public_system in re.findall(pattern, text)]
    deps = sorted({urldefrag(urljoin(context, uri))[0] for context, uri in relative})
    if any(urlparse(uri).scheme not in ("http", "https") for uri in deps):
        raise ValueError("non-HTTP schema dependency requires explicit archive review")
    return deps


def extra_dependencies(data, final, kind):
    deps = schema_dependencies(data, final)
    if "css" in kind:
        css = data.decode("utf-8")
        deps += [urldefrag(urljoin(final, s))[0] for s in re.findall(r"url\(\s*['\"]?([^)'\"\s]+)", css) if not s.startswith("data:")]
        deps += [urljoin(final, s) for s in re.findall(r"@import\s+['\"]([^'\"]+)", css)]
    if final.startswith("https://api.github.com/repos/w3c/epub-specs/issues"):
        issues = json.loads(data)
        if not isinstance(issues, list):
            raise ValueError("GitHub API did not return an issue/comment list")
        if len(issues) == 100:
            page = int(re.search(r"[?&]page=(\d+)", final).group(1))
            deps.append(re.sub(r"([?&]page=)\d+", lambda m: m.group(1) + str(page + 1), final))
        for issue in issues:
            if issue.get("comments", 0):
                deps.append(issue["comments_url"] + "?per_page=100&page=1")
    return deps


def fetch(root, bootstrap=False, complete_schemas=False):
    """First freeze creates a lock; later fetch checks it, never refreshes it."""
    manifest_path = root / "manifest.json"
    if manifest_path.exists() and not complete_schemas:
        lock = json.loads(manifest_path.read_text())
        failures = []
        for entry in lock["assets"]:
            try:
                data, final, _ = download(entry["requestUrl"])
                if sha(data) != entry["sha256"] or final != entry["finalUrl"]:
                    raise ValueError(f"upstream drift: {entry['id']}; old evidence NOT overwritten")
                local = root / entry["path"]
                if local.exists() and sha(local.read_bytes()) != entry["sha256"]:
                    raise ValueError(f"local drift: {entry['path']}; NOT overwritten")
                local.parent.mkdir(parents=True, exist_ok=True)
                local.write_bytes(data)
            except ValueError as exc:
                failures.append(str(exc))
        if failures:
            raise ValueError("\n".join(failures))
        return
    if complete_schemas:
        lock = json.loads(manifest_path.read_text())
        if lock["baseline"] != "EPUB3.3-REC2026-01-13" or lock["failures"]:
            raise ValueError("cannot extend invalid archive baseline")
        entries = lock["assets"]
        seen = {e["requestUrl"]: e for e in entries}
        queue = []
        for entry in entries:
            path = Path(entry["path"])
            if path.is_absolute() or ".." in path.parts:
                raise ValueError("unsafe archived path")
            data = (root / path).read_bytes()
            if sha(data) != entry["sha256"] or len(data) != entry["bytes"]:
                raise ValueError("cannot extend modified source")
            if "html" in entry["contentType"]:
                html_ok(data, entry["finalUrl"], requested_url=entry["requestUrl"])
            deps = schema_dependencies(data, entry["finalUrl"])
            if Path(urlparse(entry["requestUrl"]).path).suffix.lower() in (".rnc", ".rng", ".xsd", ".dtd", ".ent"):
                entry["version"] = entry["requestUrl"]
            entry["dependencies"] = sorted(set(entry["dependencies"] + deps))
            queue += [(url, sha(url.encode())[:24], "external-dependency", "schema", [entry["id"]]) for url in deps]
        total = sum(e["bytes"] for e in entries)
        failures = []
    elif not bootstrap:
        raise ValueError("no manifest: initial capture requires --bootstrap; review before committing")
    else:
        queue = [(url, name, category, level, []) for name, category, level, url in SEEDS]
        entries, seen, failures = [], {}, []
        total = 0
    while queue:
        url, name, category, level, parents = queue.pop(0)
        if url in seen:
            seen[url]["requiredBy"] = sorted(set(seen[url]["requiredBy"] + parents))
            continue
        if len(seen) >= 300:
            raise ValueError("archive exceeds finite 300-resource budget")
        try:
            data, final, kind = download(url)
            total += len(data)
            if total > 128 << 20:
                raise ValueError("archive exceeds finite 128MiB budget")
            is_html = "text/html" in kind or "application/xhtml" in kind
            dom = html_ok(data, final, requested_url=url) if is_html else None
            extension = "json" if "json" in kind else "html"
            path = f"original/{name}.{extension}" if not parents else f"assets/{sha(url.encode())[:24]}"
            entry = {"id": name, "category": category, "normativeLevel": level,
                     "requestUrl": url, "finalUrl": final,
                     "version": url if not parents or Path(urlparse(url).path).suffix.lower() in (".rnc", ".rng", ".xsd", ".dtd", ".ent") else "display/dependency asset",
                     "downloadedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                     "path": path, "bytes": len(data), "sha256": sha(data), "contentType": kind,
                     "requiredBy": parents, "dependencies": []}
            seen[url] = entry
            entries.append(entry)
            local = root / path
            local.parent.mkdir(parents=True, exist_ok=True)
            local.write_bytes(data)
            deps = resources(dom, final) if dom else []
            deps += extra_dependencies(data, final, kind)
            entry["dependencies"] = sorted(set(deps))
            for dep in entry["dependencies"]:
                queue.append((dep, sha(dep.encode())[:24], "external-dependency", "display/schema", [name]))
        except (ValueError, UnicodeError) as exc:
            failures.append({"url": url, "reason": str(exc), "requiredBy": parents})
    write_json(manifest_path, {"schemaVersion": 1, "baseline": "EPUB3.3-REC2026-01-13",
                               "assets": entries, "failures": failures})
    if failures:
        raise ValueError(f"{len(failures)} failed required assets; see manifest failures (not complete)")


def verify(root):
    manifest = json.loads((root / "manifest.json").read_text())
    if manifest["baseline"] != "EPUB3.3-REC2026-01-13" or manifest["schemaVersion"] != 1:
        raise ValueError("baseline/schema drift")
    if manifest["failures"]:
        raise ValueError("unresolved archive failures")
    by_url = {e["requestUrl"]: e for e in manifest["assets"]}
    if len(by_url) != len(manifest["assets"]):
        raise ValueError("duplicate asset URL")
    for name, _, _, url in SEEDS:
        if url not in by_url or by_url[url]["id"] != name:
            raise ValueError(f"missing/wrong baseline: {name}")
    paths = set()
    for e in manifest["assets"]:
        p = Path(e["path"])
        if p.is_absolute() or ".." in p.parts or e["path"] in paths:
            raise ValueError("unsafe/duplicate path")
        paths.add(e["path"])
        data = (root / p).read_bytes()
        if len(data) != e["bytes"] or sha(data) != e["sha256"]:
            raise ValueError(f"hash/length mismatch: {p}")
        if any(d not in e["dependencies"] for d in extra_dependencies(data, e["finalUrl"], e["contentType"])):
            raise ValueError(f"omitted asset dependency: {p}")
        if "html" in e["contentType"]:
            dom = html_ok(data, e["finalUrl"], requested_url=e["requestUrl"])
            discovered = resources(dom, e["finalUrl"])
            if any(d not in e["dependencies"] for d in discovered):
                raise ValueError(f"omitted asset dependency: {p}")
            for n in dom.walk():
                if n.attrs.get("data-include"):
                    dep = by_url.get(urljoin(e["finalUrl"], n.attrs["data-include"]))
                    if not dep or len((root / dep["path"]).read_bytes().strip()) < 300:
                        raise ValueError(f"missing/empty included body: {p}")
        if any(d not in by_url for d in e["dependencies"]):
            raise ValueError(f"missing dependency: {p}")
    return manifest


def source_inventory(root, manifest):
    candidates, sections, references = [], [], []
    for e in manifest["assets"]:
        if not e["path"].startswith("original/") or "html" not in e["contentType"]:
            continue
        dom = DOM((root / e["path"]).read_text(encoding="utf-8")).root
        nodes = list(dom.walk())
        reference_kind = "informative"
        for n in nodes:
            if n.tag in ("h2", "h3"):
                heading = normalized(n).lower()
                # Non-Normative contains the normative substring: classify the
                # explicit negative first, including both source conventions.
                if "non-normative references" in heading or "informative references" in heading:
                    reference_kind = "informative"
                elif "normative references" in heading:
                    reference_kind = "normative"
            if n.tag in ("section",) and n.attrs.get("id"):
                heading = next((normalized(c) for c in n.walk() if c.tag in ("h1", "h2", "h3", "h4", "h5", "h6")), "")
                sections.append({"document": e["id"], "anchor": n.attrs["id"], "heading": heading,
                                 "sourceHash": e["sha256"], "domPath": n.path(), "semanticReview": "pending"})
            if n.tag == "dt" and n.attrs.get("id", "").startswith(("bib-", "biblio-")):
                dds = n.parent.children[n.parent.children.index(n) + 1:]
                dd = next((x for x in dds if isinstance(x, Node)), None)
                if dd and dd.tag == "dd":
                    links = [urljoin(e["finalUrl"], c.attrs["href"]) for c in dd.walk() if c.tag == "a" and c.attrs.get("href")]
                    normative = reference_kind == "normative" or any(a.attrs.get("id") == "normative-references" for a in n.ancestors())
                    references.append({"document": e["id"], "entry": n.attrs["id"], "kind": "normative" if normative else "informative",
                                       "citation": normalized(dd), "urls": list(dict.fromkeys(links)), "status": "not-downloaded",
                                       "sha256": None, "stage": "requires-applicability-review"})
            if e["id"] not in ("epub", "rs", "a11y"):
                continue
            # Marked BCP14 instances retain node identity and enclosing condition.
            if "rfc2119" in n.attrs.get("class", "").split():
                context = next((a for a in n.ancestors() if a.tag in ("p", "li", "dd", "td")), n)
                for occurrence, m in enumerate(KEYWORDS.finditer(normalized(n).upper()), 1):
                    candidates.append(candidate(e, n, context, "bcp14", occurrence, m.group()))
            # Explicit definition slots are separate provenance, even if BCP14 overlaps.
            if n.tag == "dt" and re.search(r"\b(usage|cardinality|required|deprecated)\b", normalized(n), re.I):
                following = n.parent.children[n.parent.children.index(n) + 1:]
                dd = next((x for x in following if isinstance(x, Node)), None)
                if dd and dd.tag == "dd":
                    candidates.append(candidate(e, dd, dd, "definition-slot", 1, normalized(n)))
    return {"schemaVersion": 1, "semanticComplete": False, "candidates": candidates,
            "sections": sections, "directReferences": references,
            "limitations": ["Unmarked normative sentences, grammar and deprecated constraints require section-by-section semantic review.",
                            "Mechanical candidates and section counts are NOT proof of normative completeness."]}


def inventory(root):
    manifest = verify(root)
    inv = source_inventory(root, manifest)
    write_json(root / "inventory.json", inv)
    archived = {e["requestUrl"]: e for e in manifest["assets"]}
    dependencies = {}
    for ref in inv["directReferences"]:
        for url in ref["urls"]:
            base = urldefrag(url)[0]
            captured = archived.get(base)
            row = dependencies.setdefault(base, {"url": base, "citations": [], "phase": "pending-applicability-review",
                                                  "status": "archived" if captured else "not-downloaded",
                                                  "sha256": captured["sha256"] if captured else None,
                                                  "versionOrAcquiredAt": captured["version"] if captured else None})
            row["citations"].append({"document": ref["document"], "entry": ref["entry"], "kind": ref["kind"]})
    write_json(root / "external-dependencies.json", {"schemaVersion": 1, "recursive": False,
                                                   "applicabilityReviewComplete": False,
                                                   "dependencies": list(dependencies.values())})
    mapping = root / "matrix.json"
    if not mapping.exists():
        write_json(mapping, {"schemaVersion": 1, "semanticComplete": False,
                            "manualConstraints": [],
                            "sectionReviews": [{"document": s["document"], "domPath": s["domPath"],
                                                "anchor": s["anchor"], "sourceHash": s["sourceHash"],
                                                "status": "pending", "reviewer": None, "notes": None}
                                               for s in inv["sections"] if s["document"] in ("epub", "rs", "a11y")],
                            "rows": [
            {**c, "decision": "pending", "reason": "Awaiting section-level applicability/constraint review",
             "applicability": "not-reviewed", "phase": "not-assigned", "platform": "not-tested",
             **{dim: "not-tested" for dim in ("preserve", "parse", "edit", "render", "validate")},
             "testIds": [], "evidence": [], "gap": "No implementation support claimed by this inventory"} for c in inv["candidates"]]})
    lines = ["# EPUB 3.3 offline source index", "", "Raw files preserve upstream bytes and copyright. Links below are local; no scripts need execution.", ""]
    for e in manifest["assets"]:
        if e["path"].startswith("original/") and "html" in e["contentType"]:
            text_path = f"text/{e['id']}.txt"
            (root / "text").mkdir(exist_ok=True)
            dom = DOM((root / e["path"]).read_text(encoding="utf-8")).root
            text = dom.text()
            by_url = {a["requestUrl"]: a for a in manifest["assets"]}
            for n in dom.walk():
                if n.attrs.get("data-include"):
                    dep = by_url[urljoin(e["finalUrl"], n.attrs["data-include"])]
                    text += "\n\nINCLUDED BODY: " + dep["requestUrl"] + "\n" + (root / dep["path"]).read_text(encoding="utf-8")
            (root / text_path).write_text(text, encoding="utf-8")
            lines.append(f"- {e['id']}: [raw]({e['path']}) / [offline text]({text_path}); SHA-256 `{e['sha256']}`")
    (root / "INDEX.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    matrix = json.loads(mapping.read_text())
    table = ["# Mechanical EPUB 3.3 candidate matrix — semantic review pending", "",
             "Generated from matrix.json. `not-tested` is not `supported`, and pending candidates are not mapped constraints.",
             "Full condition/excerpt/UTF-8 hash/DOM identity remain in [matrix.json](matrix.json); no repeated instance is collapsed.", "",
             "| Instance | Document / section | Level | Decision / phase | preserve / parse / edit / render / validate |",
             "| --- | --- | --- | --- | --- |"]
    for number, r in enumerate(matrix["rows"], 1):
        states = " / ".join(r[k] for k in ("preserve", "parse", "edit", "render", "validate"))
        table.append(f"| {number} `{r['kind']}` / {r['occurrence']} | [{r['document']} / {r['specSection']}](original/{r['document']}.html#{r['specSection']}) | {r['normativeLevel']} | {r['decision']} / {r['phase']} | {states} |")
    (root / "MATRIX.md").write_text("\n".join(table) + "\n", encoding="utf-8")
    derived_paths = ["inventory.json", "external-dependencies.json", "INDEX.md", "MATRIX.md"]
    derived_paths += sorted(str(p.relative_to(root)) for p in (root / "text").glob("*.txt"))
    write_json(root / "derived.json", {"schemaVersion": 1,
               "sourceManifestSHA256": sha((root / "manifest.json").read_bytes()),
               "matrixSHA256": sha(mapping.read_bytes()),
               "files": [{"path": p, "sha256": sha((root / p).read_bytes())} for p in derived_paths]})


def candidate(e, node, context, kind, occurrence, level):
    snippet = normalized(context)
    return {"featureId": f"{e['id']}:{kind}:{node.path()}:{occurrence}", "document": e["id"],
            "specVersion": e["requestUrl"], "sourceHash": e["sha256"], "specSection": node.section(),
            "domPath": node.path(), "kind": kind, "occurrence": occurrence, "normativeLevel": level,
            "excerpt": snippet, "excerptSHA256": sha(snippet.encode())}


def verify_mapping(root, gate=False):
    inv = json.loads((root / "inventory.json").read_text())
    matrix = json.loads((root / "matrix.json").read_text())
    actual = source_inventory(root, verify(root))
    if any(inv[k] != actual[k] for k in ("candidates", "sections", "directReferences")):
        raise ValueError("inventory provenance differs from archived source")
    candidates = {c["featureId"]: c for c in inv["candidates"]}
    manual = matrix.get("manualConstraints", [])
    source_entries = {e["id"]: e for e in verify(root)["assets"]}
    for c in manual:
        entry = source_entries[c["document"]]
        dom = DOM((root / entry["path"]).read_text(encoding="utf-8")).root
        node = next((n for n in dom.walk() if n.path() == c["domPath"]), None)
        if (not node or c["sourceHash"] != entry["sha256"] or not c["excerpt"] or
                c["excerpt"] not in normalized(node) or sha(c["excerpt"].encode()) != c["excerptSHA256"] or
                c["featureId"] in candidates):
            raise ValueError("manual constraint provenance/identity mismatch")
        candidates[c["featureId"]] = c
    rows = {r["featureId"]: r for r in matrix["rows"]}
    if len(rows) != len(matrix["rows"]) or set(rows) != set(candidates):
        raise ValueError("missing/duplicate/extra candidate mapping")
    allowed = {"supported", "partial", "unsupported", "policy-disabled", "not-tested"}
    for key, row in rows.items():
        if any(row.get(k) != v for k, v in candidates[key].items()):
            raise ValueError(f"candidate provenance drift: {key}")
        if any(row.get(k) not in allowed for k in ("preserve", "parse", "edit", "render", "validate")):
            raise ValueError(f"invalid capability dimension: {key}")
        if row["decision"] not in ("mapped", "excluded", "pending") or not row["reason"]:
            raise ValueError(f"invalid decision/reason: {key}")
        if row["decision"] == "mapped" and (row["phase"] == "not-assigned" or
                row["applicability"] == "not-reviewed" or not (row["gap"] or row["evidence"])):
            raise ValueError(f"mapped constraint without stage/applicability/gap: {key}")
        if "supported" in [row[k] for k in ("preserve", "parse", "edit", "render", "validate")] and not row["evidence"]:
            raise ValueError(f"unsupported support claim without evidence: {key}")
    expected_sections = {(s["document"], s["domPath"]): s for s in inv["sections"] if s["document"] in ("epub", "rs", "a11y")}
    reviews = matrix.get("sectionReviews", [])
    if {(s["document"], s["domPath"]) for s in reviews} != set(expected_sections) or len(reviews) != len(expected_sections):
        raise ValueError("missing/duplicate section review record")
    for r in reviews:
        original = expected_sections[r["document"], r["domPath"]]
        if r["sourceHash"] != original["sourceHash"] or r["anchor"] != original["anchor"]:
            raise ValueError("section review provenance mismatch")
        if r["status"] not in ("pending", "complete"):
            raise ValueError("invalid section review status")
        if r["status"] == "complete" and (not r["reviewer"] or not r["notes"]):
            raise ValueError("section review completion without reviewer/reason")
    counts = collections.Counter(r["decision"] for r in rows.values())
    if gate and (counts["pending"] or not matrix["semanticComplete"] or any(s["status"] != "complete" for s in reviews)):
        raise ValueError("S0 semantic gate blocked: pending clause/section review (not an asset PASS)")
    return {"candidates": len(candidates), **dict(counts), "semanticComplete": matrix["semanticComplete"]}


def verify_derived(root):
    derived = json.loads((root / "derived.json").read_text())
    manifest = json.loads((root / "manifest.json").read_text())
    expected = {"inventory.json", "external-dependencies.json", "INDEX.md", "MATRIX.md"}
    expected.update(f"text/{e['id']}.txt" for e in manifest["assets"]
                    if e["path"].startswith("original/") and "html" in e["contentType"])
    if len(derived["files"]) != len(expected) or {e["path"] for e in derived["files"]} != expected:
        raise ValueError("missing/duplicate derivative mapping")
    if (derived["sourceManifestSHA256"] != sha((root / "manifest.json").read_bytes()) or
            derived["matrixSHA256"] != sha((root / "matrix.json").read_bytes())):
        raise ValueError("derived inputs changed; rerun offline index")
    for entry in derived["files"]:
        path = Path(entry["path"])
        if path.is_absolute() or ".." in path.parts or sha((root / path).read_bytes()) != entry["sha256"]:
            raise ValueError("derived hash/path mismatch")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("command", choices=("fetch", "verify", "index", "gate"))
    p.add_argument("--root", type=Path, default=ROOT)
    p.add_argument("--bootstrap", action="store_true")
    p.add_argument("--complete-schemas", action="store_true", help="extend a frozen archive only with missing schema dependencies")
    args = p.parse_args()
    try:
        if args.command == "fetch":
            fetch(args.root, args.bootstrap, args.complete_schemas)
        elif args.command == "index":
            inventory(args.root)
        else:
            archive = verify(args.root)
            counts = verify_mapping(args.root, args.command == "gate")
            verify_derived(args.root)
            print(json.dumps({"archiveFiles": len(archive["assets"]), "mapping": counts}, sort_keys=True))
    except (ValueError, OSError, KeyError, json.JSONDecodeError) as exc:
        p.exit(1, f"ERROR: {exc}\n")


if __name__ == "__main__":
    main()
