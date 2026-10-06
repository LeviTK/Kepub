"""Independent S0 acceptance regressions; run from the S0 repository root."""
import json
from pathlib import Path
import re
import sys
import tempfile
import unittest
from unittest.mock import patch
from urllib.parse import urljoin

sys.path.insert(0, str(Path("scripts").resolve()))
import epub33_assets as assets
import epub33_tests as official


class ParentAcceptance(unittest.TestCase):
    def test_missing_source_cannot_be_hidden_by_empty_failure_list(self):
        root = Path("docs/specs/epub-3.3").resolve()
        with tempfile.TemporaryDirectory() as tmp:
            copy = Path(tmp)
            (copy / "official-tests").mkdir()
            (copy / "original").symlink_to(root / "original", target_is_directory=True)
            for file in (root / "official-tests").iterdir():
                if file.name != "index.json":
                    (copy / "official-tests" / file.name).symlink_to(file, target_is_directory=file.is_dir())
            data = json.loads((root / "official-tests/index.json").read_text())
            row = next(r for r in data["cases"] if r["id"] == "pub-data-urls_browsing-context")
            for key in ("sourceCommit", "sourceArchivePath", "sourceArchiveSHA256", "generatedArtifactPath", "generatedSHA256", "websiteArtifactPath", "websiteSHA256"):
                row[key] = None
            data["failures"] = []
            assets.write_json(copy / "official-tests/index.json", data)
            with self.assertRaises(ValueError):
                official.verify(copy)

    def test_actual_rnc_includes_are_archived(self):
        root = Path("docs/specs/epub-3.3")
        manifest = json.loads((root / "manifest.json").read_text())
        urls = {e["requestUrl"] for e in manifest["assets"]}
        missing = []
        for entry in manifest["assets"]:
            if entry["requestUrl"].endswith(".rnc"):
                text = (root / entry["path"]).read_text()
                for relative in re.findall(r'(?m)^\s*(?:include|external)\s+"([^"]+)"', text):
                    target = urljoin(entry["finalUrl"], relative)
                    if target not in urls:
                        missing.append(target)
        self.assertEqual(missing, [], "Required schema includes are not external reference bibliography")

    def test_fixed_version_redirect_must_not_accept_new_rec(self):
        requested = "https://example.test/2026/REC-demo-20260113/"
        redirected = "https://example.test/2027/REC-demo-20270113/"
        body = (f'<html><head><title>Changed Recommendation</title></head><body>'
                f'<h1>Changed Recommendation</h1><dl><dt>This version:</dt>'
                f'<dd><a href="{redirected}">{redirected}</a></dd></dl>'
                '<section id="normative"><h2>New edition</h2><p>This is substantive '
                'text in a newer recommendation, not a CAPTCHA, access failure, or '
                'empty dynamic shell. It must never inherit the old fixed baseline '
                'merely because an HTTP request started at the old URL.</p></section>'
                '</body></html>').encode()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with patch.object(assets, "SEEDS", [("epub", "normative", "REC", requested)]), patch.object(assets, "download", return_value=(body, redirected, "text/html")):
                with self.assertRaises(ValueError):
                    assets.fetch(root, bootstrap=True)
                    assets.inventory(root)
                    assets.verify(root)


if __name__ == "__main__":
    unittest.main(verbosity=2)
