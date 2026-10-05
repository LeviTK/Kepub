"""Offline adversarial tests; no network, product changes or private EPUBs."""
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import epub33_assets as assets
import epub33_tests as official


URL = "https://example.test/2026/REC-demo-20260113/"
BODY = (f'<html><head><title>Fixture</title></head><body><h1>Fixture</h1>'
        f'<dl><dt>This version:</dt><dd><a href="{URL}">{URL}</a></dd></dl>'
        '<section id="one"><h2>Requirements</h2><p>The publication '
        '<em class="rfc2119">MUST NOT</em> load a remote entity, and '
        '<em class="rfc2119">MUST</em> preserve the original source. '
        'This asymmetric constraint supplies enough real text for a document, '
        'rather than a script-only dynamic shell with a placeholder title.</p>'
        '<dl><dt>Usage:</dt><dd>Exactly one required identifier.</dd></dl>'
        '</section></body></html>').encode()


class Integrity(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.seed = patch.object(assets, "SEEDS", [("epub", "normative", "REC", URL)])
        self.seed.start()
        self.addCleanup(self.seed.stop)
        self.capture(BODY)

    def capture(self, body):
        with patch.object(assets, "download", return_value=(body, URL, "text/html")):
            assets.fetch(self.root, bootstrap=True)
        assets.inventory(self.root)

    def read(self, name):
        return json.loads((self.root / name).read_text())

    def save(self, name, data):
        assets.write_json(self.root / name, data)

    def test_candidate_instances_and_definition_do_not_collapse(self):
        inv = self.read("inventory.json")
        self.assertEqual([c["normativeLevel"] for c in inv["candidates"]], ["MUST NOT", "MUST", "Usage:"])
        self.assertEqual(len({c["featureId"] for c in inv["candidates"]}), 3)
        self.assertEqual(assets.verify_mapping(self.root)["pending"], 3)
        with self.assertRaisesRegex(ValueError, "semantic gate blocked"):
            assets.verify_mapping(self.root, gate=True)

    def test_missing_file(self):
        (self.root / "original/epub.html").unlink()
        with self.assertRaises(FileNotFoundError):
            assets.verify(self.root)

    def test_wrong_hash(self):
        with (self.root / "original/epub.html").open("ab") as f:
            f.write(b"x")
        with self.assertRaisesRegex(ValueError, "hash/length mismatch"):
            assets.verify(self.root)

    def test_version_drift_with_recomputed_hash_still_rejected(self):
        body = BODY.replace(URL.encode(), b"https://example.test/2027/REC-demo-20270113/")
        (self.root / "original/epub.html").write_bytes(body)
        m = self.read("manifest.json")
        m["assets"][0].update(sha256=assets.sha(body), bytes=len(body))
        self.save("manifest.json", m)
        with self.assertRaisesRegex(ValueError, "fixed version identity missing"):
            assets.verify(self.root)

    def test_empty_dynamic_shell(self):
        with self.assertRaisesRegex(ValueError, "empty/dynamic shell"):
            assets.html_ok(b'<html><h1>Loading</h1><script>/*' + b'x'*1000 + b'*/</script></html>', URL)

    def test_missing_include_is_not_complete(self):
        body = b'<html><div data-include="body.md"></div></html>'
        (self.root / "original/epub.html").write_bytes(body)
        m = self.read("manifest.json")
        m["assets"][0].update(sha256=assets.sha(body), bytes=len(body))
        self.save("manifest.json", m)
        with self.assertRaises(ValueError):
            assets.verify(self.root)

    def test_omitted_mapping(self):
        m = self.read("matrix.json")
        m["rows"].pop()
        self.save("matrix.json", m)
        with self.assertRaisesRegex(ValueError, "candidate mapping"):
            assets.verify_mapping(self.root)

    def test_inventory_and_mapping_cannot_both_hide_clause(self):
        i, m = self.read("inventory.json"), self.read("matrix.json")
        i["candidates"].pop()
        m["rows"].pop()
        self.save("inventory.json", i)
        self.save("matrix.json", m)
        with self.assertRaisesRegex(ValueError, "inventory provenance"):
            assets.verify_mapping(self.root)

    def test_refresh_refuses_upstream_drift_without_overwriting(self):
        before = (self.root / "original/epub.html").read_bytes()
        manifest = (self.root / "manifest.json").read_bytes()
        with patch.object(assets, "download", return_value=(BODY + b"changed", URL, "text/html")):
            with self.assertRaisesRegex(ValueError, "upstream drift"):
                assets.fetch(self.root)
        self.assertEqual((self.root / "original/epub.html").read_bytes(), before)
        self.assertEqual((self.root / "manifest.json").read_bytes(), manifest)

    def test_css_optional_end_tags_are_siblings(self):
        dom = assets.DOM('<dl><dt>a<dd>alpha<dt>b<dd>beta</dl>').root
        dl = next(n for n in dom.walk() if n.tag == "dl")
        self.assertEqual([(n.tag, n.text()) for n in dl.children], [("dt", "a"), ("dd", "alpha"), ("dt", "b"), ("dd", "beta")])

    def test_section_review_cannot_be_omitted(self):
        matrix = self.read("matrix.json")
        matrix["sectionReviews"].pop()
        self.save("matrix.json", matrix)
        with self.assertRaisesRegex(ValueError, "section review record"):
            assets.verify_mapping(self.root)

    def test_supported_claim_requires_evidence(self):
        matrix = self.read("matrix.json")
        matrix["rows"][0]["parse"] = "supported"
        self.save("matrix.json", matrix)
        with self.assertRaisesRegex(ValueError, "support claim without evidence"):
            assets.verify_mapping(self.root)

    def test_derived_text_corruption(self):
        (self.root / "text/epub.txt").write_text("wrong source")
        with self.assertRaisesRegex(ValueError, "derived hash"):
            assets.verify_derived(self.root)

    def test_derived_mapping_omission(self):
        derived = self.read("derived.json")
        derived["files"].pop()
        self.save("derived.json", derived)
        with self.assertRaisesRegex(ValueError, "derivative mapping"):
            assets.verify_derived(self.root)

    def test_css_dependency_cannot_be_hidden_from_manifest(self):
        url = URL + "style.css"
        body = BODY.replace(b"</head>", b'<link rel="stylesheet" href="style.css"></head>')
        css = b'@import "required.css";'
        (self.root / "original/epub.html").write_bytes(body)
        (self.root / "assets").mkdir()
        (self.root / "assets/style.css").write_bytes(css)
        manifest = self.read("manifest.json")
        manifest["assets"][0].update(sha256=assets.sha(body), bytes=len(body), dependencies=[url])
        manifest["assets"].append({"path": "assets/style.css", "requestUrl": url, "finalUrl": url,
                                   "contentType": "text/css", "sha256": assets.sha(css),
                                   "bytes": len(css), "dependencies": []})
        self.save("manifest.json", manifest)
        with self.assertRaisesRegex(ValueError, "omitted asset dependency"):
            assets.verify(self.root)


class OfficialArtifacts(unittest.TestCase):
    def test_zip_generation_is_reproducible_and_stores_mimetype_first(self):
        files = {"z.xhtml": b"asymmetric last", "mimetype": b"application/epub+zip", "a.xhtml": b"first"}
        artifact = official.generate(files)
        self.assertEqual(artifact, official.generate(dict(reversed(list(files.items())))))
        self.assertEqual(official.zip_files(artifact), files)
        with official.zipfile.ZipFile(io.BytesIO(artifact)) as z:
            self.assertEqual(z.namelist(), ["mimetype", "a.xhtml", "z.xhtml"])
            self.assertEqual(z.infolist()[0].compress_type, official.zipfile.ZIP_STORED)

    def test_duplicate_tar_source_is_not_silently_overwritten(self):
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode="w") as tar:
            for payload in (b"original", b"replacement"):
                member = tarfile.TarInfo("chapter.xhtml")
                member.size = len(payload)
                tar.addfile(member, io.BytesIO(payload))
        with self.assertRaisesRegex(ValueError, "duplicate source"):
            official.source_files(data.getvalue())

    def test_official_report_case_cannot_be_omitted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "original").mkdir()
            (root / "original/test-index.html").write_text('<table><tr><td id="one">one</td><td>true</td><td>MUST</td><td>condition</td><td><a href="#constraint">spec</a></td></tr></table>')
            assets.write_json(root / "official-tests/index.json", {"cases": []})
            with self.assertRaisesRegex(ValueError, "official report mapping"):
                official.verify(root)

    def test_empty_source_cannot_be_reported_as_zero_gaps(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "original").mkdir()
            report = b'<table><tr><td id="one">one</td><td>true</td><td>MUST</td><td>condition</td><td><a href="#constraint">spec</a></td></tr></table>'
            (root / "original/test-index.html").write_bytes(report)
            dest = root / "official-tests"
            dest.mkdir()
            (dest / "LICENSE.upstream.md").write_bytes(b"license")
            (dest / "generateEpubs.upstream.sh").write_bytes(b"generator")
            row = {"id": "one", **official.report_cases(root)["one"],
                   "reportSHA256": assets.sha(report), "sourceCommit": None,
                   "websiteArtifactPath": None, "sourceArchivePath": None, "generatedArtifactPath": None}
            assets.write_json(dest / "index.json", {"cases": [row], "failures": [],
                 "licenseSHA256": assets.sha(b"license"), "upstreamGeneratorSHA256": assets.sha(b"generator")})
            with self.assertRaisesRegex(ValueError, "unresolved official source"):
                official.verify(root)


if __name__ == "__main__":
    unittest.main()
