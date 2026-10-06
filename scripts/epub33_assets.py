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


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "docs/specs/epub-3.3"
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
            if not href.startswith("#") and (re.search(r"\.(?:rng|rnc|xsd|dtd|ent|nvdl|sch)(?:$|\?)", target) or
                    (target.startswith(base) and target != base and urlparse(target).path.endswith(".html")) or
                    (base.startswith("https://w3c.github.io/epub-specs/epub33/reports/") and
                     target.startswith("https://w3c.github.io/epub-specs/epub33/reports/") and target.endswith(".html"))):
                raw = href
        if raw:
            target = urldefrag(urljoin(base, raw))[0]
            schema = re.fullmatch(r"https://github.com/w3c/epubcheck/(?:blob|tree)/(?:main|master)/(.+)", target)
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
    elif suffix in (".rng", ".xsd", ".nvdl", ".sch"):
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
            elif node.tag == "{http://purl.oclc.org/dsdl/nvdl/ns/structure/1.0}validate":
                if not node.get("schema"):
                    raise ValueError("NVDL validate missing schema")
                relative.append((context, node.get("schema")))
            elif node.tag == "{http://purl.oclc.org/dsdl/schematron}include":
                if not node.get("href"):
                    raise ValueError("Schematron include missing href")
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
    schema_root = f"https://raw.githubusercontent.com/w3c/epubcheck/{EPUBCHECK_SCHEMA_COMMIT}/src/main/resources/com/adobe/epubcheck/schema/30/"
    if final.startswith(schema_root) and Path(urlparse(final).path).suffix.lower() in (".rnc", ".rng", ".nvdl", ".sch"):
        # This directory has its own IDPF MIT-form notice, not just root BSD.
        deps.append(schema_root + "LICENSE")
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
                dom = html_ok(data, entry["finalUrl"], requested_url=entry["requestUrl"])
                deps = resources(dom, entry["finalUrl"])
            else:
                deps = []
            deps += extra_dependencies(data, entry["finalUrl"], entry["contentType"])
            if Path(urlparse(entry["requestUrl"]).path).suffix.lower() in (".rnc", ".rng", ".xsd", ".dtd", ".ent", ".nvdl", ".sch"):
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
                     "version": url if not parents or Path(urlparse(url).path).suffix.lower() in (".rnc", ".rng", ".xsd", ".dtd", ".ent", ".nvdl", ".sch") else "display/dependency asset",
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
                                       "sourceHash": e["sha256"], "domPath": dd.path(),
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
    non_url = []
    for ref in inv["directReferences"]:
        if not ref["urls"]:
            non_url.append({k: ref[k] for k in ("document", "entry", "kind", "citation", "sourceHash", "domPath")})
            non_url[-1].update(url=None, sha256=None, status="not-downloaded",
                               versionOrAcquiredAt=None, phase="pending-applicability-review")
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
                                                   "dependencies": list(dependencies.values()),
                                                   "nonURLReferences": non_url})
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
    table = ["# EPUB 3.3 source clause matrix — S0 acceptance pending", "",
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


def repository_evidence(reference):
    """Resolve a file or Go test reference; existence alone proves no behavior."""
    if not isinstance(reference, str):
        raise ValueError("invalid repository evidence reference")
    name, separator, test = reference.partition(":")
    path = Path(name)
    if (not name or path.is_absolute() or ".." in path.parts or
            not (PROJECT / path).resolve().is_relative_to(PROJECT.resolve()) or
            not (PROJECT / path).is_file()):
        raise ValueError(f"missing/unsafe repository evidence: {reference}")
    if separator and (not re.fullmatch(r"Test[A-Za-z0-9_]+", test) or
                      not name.endswith("_test.go") or
                      not re.search(rf"(?m)^func\s+{re.escape(test)}\(\s*\w+\s+\*testing\.T\s*\)",
                                    (PROJECT / path).read_text())):
        raise ValueError(f"missing repository test evidence: {reference}")
    return PROJECT / path


def acceptance_inputs(root):
    """Identity of source, mapping, indexes, review inputs and implementation.

    Approval records are deliberately not inputs to their own identity. This
    is repository review consistency, not a signature or access-control scheme.
    """
    files = {p for p in root.rglob("*") if p.is_file() and p != root / "acceptance.json"}
    # Original bytes and upstream licenses are included, not just their indexes.
    inputs = {"archive/" + str(p.relative_to(root)): sha(p.read_bytes()) for p in sorted(files)}
    for directory in ("scripts", "cmd", "internal"):
        for path in sorted((PROJECT / directory).rglob("*")):
            if path.is_file() and path.suffix in (".py", ".go"):
                inputs["repository/" + str(path.relative_to(PROJECT))] = sha(path.read_bytes())
    for name in ("go.mod", "go.sum", ".agents/setup", "README.md", "docs/DEVELOPMENT_PLAN.md",
                 "docs/CLI_CONTRACT.md", "docs/EPUB33_SUPPORT_MATRIX.md",
                 "docs/verification/S0_EPUBCHECK_2026.md"):
        inputs["repository/" + name] = sha((PROJECT / name).read_bytes())
    for path in sorted((PROJECT / "docs/verification").glob("S0_ASSETS*.md")):
        inputs["repository/" + str(path.relative_to(PROJECT))] = sha(path.read_bytes())
    matrix = json.loads((root / "matrix.json").read_text())
    for row in matrix.get("rows", []):
        for reference in row["evidence"]:
            path = repository_evidence(reference)
            inputs["repository/" + str(path.relative_to(PROJECT))] = sha(path.read_bytes())
    return inputs


def verify_acceptance(root):
    """Aggregate S0 validators and current-input parent/actual Droid decisions."""
    import epub33_semantics as semantics
    import epub33_upstreams as upstreams
    verify_derived(root)
    expected = semantics.build(root)
    if json.loads((root / "semantic-index.json").read_text()) != expected:
        raise ValueError("S0 semantic gate blocked: semantic index drift")
    upstreams.verify(root / "upstreams")
    path = root / "acceptance.json"
    if not path.is_file():
        raise ValueError("S0 semantic gate blocked: independent acceptance records missing")
    acceptance = json.loads(path.read_text())
    inputs = acceptance_inputs(root)
    identity = sha(json.dumps(inputs, sort_keys=True, separators=(",", ":")).encode())
    if acceptance.get("schemaVersion") != 1 or acceptance.get("inputs") != inputs:
        raise ValueError("S0 semantic gate blocked: approval input identity changed")
    reviews = acceptance.get("reviews", {})
    if set(reviews) != {"parent", "droid"}:
        raise ValueError("S0 semantic gate blocked: parent/Droid acceptance incomplete")
    for role, review in reviews.items():
        if (review.get("decision") != "approved" or review.get("scope") != "complete-S0" or
                not review.get("reviewer") or review.get("inputIdentitySHA256") != identity):
            raise ValueError(f"S0 semantic gate blocked: {role} has not approved these inputs")
        report = repository_evidence(review["reportPath"])
        if sha(report.read_bytes()) != review["reportSHA256"]:
            raise ValueError(f"S0 semantic gate blocked: {role} report changed")
        decisions = re.findall(r"```json\s*(.*?)\s*```", report.read_text(), re.S)
        if len(decisions) != 1:
            raise ValueError(f"S0 semantic gate blocked: actual {role} decision missing")
        decision = json.loads(decisions[0])
        if (not isinstance(decision, dict) or decision.get("scope") != "complete-S0" or
                decision.get("decision") != "approved" or decision.get("findings") != [] or
                decision.get("inputIdentitySHA256") != identity or
                (role == "droid" and decision.get("readingComplete") is not True)):
            name = "Droid" if role == "droid" else "parent"
            raise ValueError(f"S0 semantic gate blocked: actual {name} review not complete/approved")
    review = reviews["droid"]
    report = repository_evidence(review["reportPath"])
    stream = repository_evidence(review["streamPath"])
    exit_file = repository_evidence(review["exitPath"])
    if (sha(stream.read_bytes()) != review["streamSHA256"] or
            sha(exit_file.read_bytes()) != review["exitSHA256"] or exit_file.read_text().strip() != "0" or
            review.get("cliVersion") != "0.233.0"):
        raise ValueError("S0 semantic gate blocked: Droid execution evidence mismatch")
    events = [json.loads(line) for line in stream.read_text().splitlines() if line.strip()]
    starts = [e for e in events if e.get("type") == "system" and e.get("subtype") == "init"]
    ends = [e for e in events if e.get("type") == "completion"]
    if (len(starts) != 1 or len(ends) != 1 or starts[0].get("model") != "claude-opus-5-5" or
            starts[0].get("reasoning_effort") != "medium" or not starts[0].get("session_id") or
            starts[0]["session_id"] != ends[0].get("session_id") or
            starts[0]["session_id"] != review.get("sessionId")):
        raise ValueError("S0 semantic gate blocked: Droid init/completion identity mismatch")
    if report.read_text().strip() != ends[0]["finalText"].strip():
        raise ValueError("S0 semantic gate blocked: Droid report is not actual completion")
    # The decisions must be in the actual reports, not only coordinator-written
    # receipt fields. R1's rejection/incomplete reading cannot satisfy this.
    return identity


def non_normative(node):
    for ancestor in node.ancestors():
        if set(ancestor.attrs.get("class", "").split()) & {"informative", "note", "example"}:
            return True
        if ancestor.tag == "details" and any(
                isinstance(c, Node) and c.tag == "summary" and normalized(c) == "Explanation"
                for c in ancestor.children):
            return True
        if ancestor.tag == "ol" and ancestor.parent:
            preceding = ancestor.parent.children[:ancestor.parent.children.index(ancestor)]
            previous = next((c for c in reversed(preceding) if isinstance(c, Node)), None)
            if previous and "pseudo-code exemplifies the obfuscation algorithm" in normalized(previous):
                return True
    return False


def source_member_text(node):
    """Formal statement text, without subordinate lists or Explanation material."""
    def text(n):
        if non_normative(n):
            return ""
        return "".join(c if isinstance(c, str) else text(c)
                       for c in n.children if isinstance(c, str) or
                       c.tag not in ("ol", "ul", "script", "style"))
    return " ".join(text(node).split())


def reconcile_source_members(source_nodes, rows, reviews):
    """Finite source families, independently of surviving/manual review rows.

    This is structural reconciliation of reviewed formal algorithms and named
    list/value families, not a classifier for all natural-language obligations.
    Selectors address existing frozen DOMs, not a second hand-maintained ledger.
    """
    families = {
        "epub": {
            "sec-container-filenames": ("ul[1]/li[3]/ul[1]",),
            "sec-data-urls": ("ul[1]",), "sec-encryption.xml-encryption": ("ul[1]",),
            "sec-item-resource-properties": ("ul[1]",), "sec-resource-locations": ("ul[1]",),
            "sec-xhtml-custom-attributes": ("ul[1]",), "sec-foreign-resources": ("ul[1]",),
            "sec-nav-toc": ("ul[1]",), "sec-skippability": ("ul[1]",),
            "sec-escapability": ("ul[1]",), "sec-container-iri": ("ul[1]",),
            "sec-property-datatype": ("ul[1]",), "sec-nav-def-model": ("ul[1]",),
            "sec-alternate": ("table[1]/tbody[1]/tr[2]/td[1]/ul[1]",),
            **{anchor: ("dl[1]",) for anchor in (
                "page-spread", "layout", "layout-overrides", "orientation", "orientation-overrides",
                "spread", "spread-overrides", "flow", "flow-overrides", "sec-exempt-resources")},
            "obfus-algorithm": ("p[1]", "p[2]", "p[3]"),
        },
        "a11y": {"sec-page-nav-applicability": ("ul[1]",),
                 "sec-sync-order": ("dl[1]/dd[3]/ul[1]",)},
    }
    status = {(r["document"], r["domPath"]): r["status"] for r in reviews}
    by_path = collections.defaultdict(list)
    for row in rows.values():
        by_path[row["document"], row["domPath"]].append(row)
    for document, nodes in source_nodes.items():
        anchors = {n.attrs["id"]: n for n in nodes.values() if n.tag == "section" and n.attrs.get("id")}
        members = {}
        for node in nodes.values():
            if node.tag != "ol" or "algorithm" not in node.attrs.get("class", "").split() or non_normative(node):
                continue
            for step in node.walk():
                if step.tag != "li" or non_normative(step):
                    continue
                paragraphs = [c for c in step.children if isinstance(c, Node) and c.tag == "p"]
                for member in paragraphs or [step]:
                    text = source_member_text(member)
                    if text:
                        members[member.path()] = (member, text)
        for anchor, suffixes in families.get(document, {}).items():
            if anchor not in anchors:
                continue  # Small synthetic fixtures need not contain the full REC.
            for suffix in suffixes:
                container = nodes.get(anchors[anchor].path() + "/" + suffix)
                if container is None:
                    raise ValueError(f"reviewed source family selector drift: {document}:{anchor}/{suffix}")
                selected = ([n for n in container.walk() if n.tag == "li" and not
                             any(c.tag == "li" for c in list(n.walk())[1:])]
                            if container.tag in ("ul", "ol") else
                            [c for c in container.children if isinstance(c, Node) and c.tag == "dd"]
                            if container.tag == "dl" else [container])
                for member in selected:
                    if not non_normative(member):
                        paragraphs = [c for c in member.children if isinstance(c, Node) and c.tag == "p"]
                        for unit in paragraphs if member.tag == "li" and paragraphs else [member]:
                            text = source_member_text(unit)
                            if text:
                                members[unit.path()] = (unit, text)
        for path, (member, text) in members.items():
            section = next(n for n in member.ancestors() if n.tag == "section")
            if status.get((document, section.path())) != "complete":
                continue
            # A direct step paragraph can be recorded as the host li, but no
            # wider ancestor or Explanation descendant can stand in for it.
            paths = [path]
            if member.tag == "p" and member.parent.tag == "li" and sum(
                    isinstance(c, Node) and c.tag == "p" for c in member.parent.children) == 1:
                paths.append(member.parent.path())
            statement_rows = [r for p in paths for r in by_path[document, p]]
            # Marked keywords are descendant instances of this exact statement,
            # not a substitute from a wider ancestor or a different sibling.
            statement_rows += [r for r in rows.values() if r["document"] == document and
                               r["kind"] == "bcp14" and r["domPath"].startswith(path + "/")]
            matches = [r for r in statement_rows
                       if r["decision"] in ("mapped", "excluded") and text and text in r["excerpt"]]
            if not matches:
                raise ValueError(f"complete section missing reviewed source member: {document}:{path}")


def verify_mapping(root, gate=False, matrix=None):
    inv = json.loads((root / "inventory.json").read_text())
    if matrix is None:
        matrix = json.loads((root / "matrix.json").read_text())
    actual = source_inventory(root, verify(root))
    if any(inv[k] != actual[k] for k in ("candidates", "sections", "directReferences")):
        raise ValueError("inventory provenance differs from archived source")
    candidates = {c["featureId"]: c for c in inv["candidates"]}
    manual = matrix.get("manualConstraints", [])
    source_entries = {e["id"]: e for e in verify(root)["assets"]}
    source_nodes = {document: {n.path(): n for n in
                              DOM((root / source_entries[document]["path"]).read_text(encoding="utf-8")).root.walk()}
                    for document in ("epub", "rs", "a11y") if document in source_entries}
    for c in manual:
        entry = source_entries[c["document"]]
        if c["document"] not in source_nodes:
            dom = DOM((root / entry["path"]).read_text(encoding="utf-8")).root
            source_nodes[c["document"]] = {n.path(): n for n in dom.walk()}
        node = source_nodes[c["document"]].get(c["domPath"])
        if (not node or c["sourceHash"] != entry["sha256"] or not c["excerpt"] or
                c["excerpt"] not in normalized(node) or sha(c["excerpt"].encode()) != c["excerptSHA256"] or
                c["featureId"] in candidates):
            raise ValueError("manual constraint provenance/identity mismatch")
        canonical = candidate(entry, node, node, c["kind"], c["occurrence"], c["normativeLevel"])
        if any(c[k] != canonical[k] for k in
               ("featureId", "document", "specVersion", "sourceHash", "specSection", "domPath")):
            raise ValueError("manual constraint provenance/identity differs from archived source")
        required_binding = {"li": "contextDOM", "dd": "termDOM"}.get(node.tag)
        if c["kind"] == "manual-amendment" and required_binding and required_binding not in c:
            raise ValueError("manual list/value constraint missing required source binding")
        for prefix in ("context", "term"):
            if prefix + "DOM" in c:
                linked = source_nodes[c["document"]].get(c[prefix + "DOM"])
                if (not linked or c[prefix + "Excerpt"] != normalized(linked) or
                        c[prefix + "SHA256"] != sha(normalized(linked).encode()) or
                        linked.section() != node.section()):
                    raise ValueError("manual inherited context/value binding differs from source")
                owners = [node] if prefix == "term" else [n for n in node.ancestors() if n.tag in ("ul", "ol")]
                preceding = []
                for owner in owners:
                    siblings = owner.parent.children[:owner.parent.children.index(owner)]
                    previous = next((n for n in reversed(siblings) if isinstance(n, Node)), None)
                    if previous:
                        preceding.append(previous)
                if linked not in preceding or (prefix == "term" and (node.tag != "dd" or linked.tag != "dt")):
                    raise ValueError("manual context/value is not the source list introduction/term")
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
        if row["decision"] == "mapped":
            document = row["document"]
            if document not in source_nodes:
                dom = DOM((root / source_entries[document]["path"]).read_text(encoding="utf-8")).root
                source_nodes[document] = {n.path(): n for n in dom.walk()}
            node = source_nodes[document][row["domPath"]]
            if non_normative(node):
                raise ValueError(f"mapped constraint inherits non-normative source scope: {key}")
        if row["decision"] == "mapped" and (row["phase"] in ("not-assigned", "S0-excluded") or
                row["applicability"] == "not-reviewed" or not (row["gap"] or row["evidence"])):
            raise ValueError(f"mapped constraint without stage/applicability/gap: {key}")
        if "supported" in [row[k] for k in ("preserve", "parse", "edit", "render", "validate")] and not row["evidence"]:
            raise ValueError(f"unsupported support claim without evidence: {key}")
        for reference in row["evidence"]:
            repository_evidence(reference)
        for test in row["testIds"]:
            if ":" not in test or test not in row["evidence"]:
                raise ValueError(f"test identity without matching repository evidence: {key}")
            repository_evidence(test)
        supported = {k for k in ("preserve", "parse", "edit", "render", "validate") if row[k] == "supported"}
        if supported:
            # Code/test references are discoverability, not executed coverage.
            coverage = set()
            for reference in row["evidence"]:
                path = repository_evidence(reference)
                if ":" in reference or path.suffix != ".json":
                    continue
                record = json.loads(path.read_text())
                if (isinstance(record, dict) and record.get("result") == "passed" and key in record.get("featureIds", []) and
                        row["testIds"] and set(row["testIds"]) <= set(record.get("testIds", []))):
                    coverage.update(record.get("dimensions", []))
            if not supported <= coverage:
                raise ValueError(f"support claim requires clause-specific execution evidence: {key}")
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
    reconcile_source_members(source_nodes, rows, reviews)
    counts = collections.Counter(r["decision"] for r in rows.values())
    if gate:
        if counts["pending"] or any(s["status"] != "complete" for s in reviews):
            raise ValueError("S0 semantic gate blocked: pending clause/section review (not an asset PASS)")
        verify_acceptance(root)
    return {"candidates": len(candidates), **dict(counts),
            "semanticComplete": True if gate else matrix["semanticComplete"]}


def import_reviews(root, paths, amendments=()):
    """Validate complete per-REC review packets before replacing any matrix bytes."""
    matrix = json.loads((root / "matrix.json").read_text())
    existing = {r["featureId"] for r in matrix["rows"]}
    documents = set()
    for path in paths:
        review = json.loads(path.read_text())
        document = review["scope"]["document"]
        if document not in ("epub", "rs", "a11y") or document in documents:
            raise ValueError("unknown/duplicate review document")
        documents.add(document)
        for key in ("rows", "manualConstraints", "sectionReviews"):
            if any(r["document"] != document for r in review[key]):
                raise ValueError("review packet crosses document ownership")
            matrix[key] = [r for r in matrix.get(key, []) if r["document"] != document] + review[key]
    entries = {e["id"]: e for e in verify(root)["assets"]}
    for path in amendments:
        review = json.loads(path.read_text())
        document = review["document"]
        if document not in ("epub", "rs", "a11y") or review["sourceHash"] != entries[document]["sha256"]:
            raise ValueError("semantic amendment source identity mismatch")
        nodes = {n.path(): n for n in DOM((root / entries[document]["path"]).read_text()).root.walk()}
        rules = []
        for rule in review["constraints"]:
            paths = rule.get("domPaths", [rule.get("domPath")])
            if "leafCount" in rule:
                container = nodes[rule["domPath"]]
                if container.tag not in ("ul", "ol"):
                    raise ValueError("inherited constraint selector is not a list")
                paths = [n.path() for n in container.walk() if n.tag == "li" and
                         not any(c.tag == "li" for c in list(n.walk())[1:])]
                if len(paths) != rule["leafCount"]:
                    raise ValueError("reviewed inherited list cardinality drift")
            rules.extend({**rule, "domPath": path} for path in paths)
        for rule in rules:
            node = nodes.get(rule["domPath"])
            if node is None:
                raise ValueError("semantic amendment source node missing")
            c = candidate(entries[document], node, node, rule.get("kind", "manual-amendment"),
                          rule.get("occurrence", 1), rule["normativeLevel"])
            if node.tag == "li" and c["kind"] == "manual-amendment" and "contextDOM" not in rule:
                owner = next(n for n in node.ancestors() if n.tag in ("ul", "ol"))
                previous = owner.parent.children[:owner.parent.children.index(owner)]
                context = next((n for n in reversed(previous) if isinstance(n, Node)), None)
                if context is None:
                    raise ValueError("list constraint lacks source introduction")
                c.update(contextDOM=context.path(), contextExcerpt=normalized(context),
                         contextSHA256=sha(normalized(context).encode()))
            elif "contextDOM" in rule:
                context = nodes[rule["contextDOM"]]
                c.update(contextDOM=context.path(), contextExcerpt=normalized(context),
                         contextSHA256=sha(normalized(context).encode()))
            if node.tag == "dd":
                previous = node.parent.children[:node.parent.children.index(node)]
                term = next((n for n in reversed(previous) if isinstance(n, Node)), None)
                if not term or term.tag != "dt":
                    raise ValueError("value definition lacks preceding term")
                c.update(termDOM=term.path(), termExcerpt=normalized(term),
                         termSHA256=sha(normalized(term).encode()))
            if "excerpt" in rule:
                c.update(excerpt=rule["excerpt"], excerptSHA256=sha(rule["excerpt"].encode()))
            row = {**c, **{k: rule[k] for k in ("reason", "applicability", "phase")},
                   "decision": rule.get("decision", "mapped"), "platform": "not-tested", "testIds": [], "evidence": [],
                   "gap": "No clause-specific executed evidence; source analysis is not implementation support",
                   **{k: "not-tested" for k in ("preserve", "parse", "edit", "render", "validate")}}
            for key, value in (("manualConstraints", c), ("rows", row)):
                position = next((i for i, r in enumerate(matrix[key]) if r["featureId"] == c["featureId"]), None)
                if position is None:
                    matrix[key].append(value)
                else:
                    matrix[key][position] = value
    if not existing <= {r["featureId"] for r in matrix["rows"]}:
        raise ValueError("import would silently delete existing review identities/candidate mapping; retain reasoned exclusions")
    # Source review alone cannot close the complete S0 gate, which also needs
    # dependency applicability, official-test mapping, and independent acceptance.
    matrix["semanticComplete"] = False
    implementation = root / "reviews/implementation.json"
    if implementation.exists():
        packet = json.loads(implementation.read_text())
        if packet["sourceHashes"] != {d: entries[d]["sha256"] for d in ("epub", "rs", "a11y")}:
            raise ValueError("implementation reference source identity mismatch")
        rows = {r["featureId"]: r for r in matrix["rows"]}
        seen = set()
        for reference in packet["rows"]:
            if reference["featureId"] not in rows or reference["featureId"] in seen:
                raise ValueError("unknown/duplicate implementation reference")
            seen.add(reference["featureId"])
            row = rows[reference["featureId"]]
            row.update(evidence=reference["evidence"], testIds=reference["testIds"], gap=reference["gap"])
    for row in matrix["rows"]:
        if row["excerpt"].rstrip().endswith(":") and "inherited list context" in row["reason"]:
            row["reason"] = ("Source-addressed introductory constraint: this excerpt ends at the colon and does not "
                             "contain the enumerated leaves. Inherited members/conditions require their separate "
                             "source mappings and the full fixed REC; this paragraph alone is not list completeness evidence.")
    counts = verify_mapping(root, matrix=matrix)
    write_json(root / "matrix.json", matrix)
    return counts


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
    p.add_argument("--review", action="append", type=Path, default=[], help="index: validate and import a complete per-REC semantic review packet")
    p.add_argument("--amendment", action="append", type=Path, default=[], help="index: import source-addressed independently reviewed missing constraints")
    args = p.parse_args()
    try:
        if (args.review or args.amendment) and args.command != "index":
            raise ValueError("semantic review inputs are only valid with offline index")
        if args.command == "fetch":
            fetch(args.root, args.bootstrap, args.complete_schemas)
        elif args.command == "index":
            if args.review or args.amendment:
                print(json.dumps(import_reviews(args.root, args.review, args.amendment), sort_keys=True))
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
