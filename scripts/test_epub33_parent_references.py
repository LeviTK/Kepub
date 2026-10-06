"""Independent normative/informative reference classification regressions."""
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path("scripts").resolve()))
import epub33_assets as assets


class ReferenceClassification(unittest.TestCase):
    def test_non_normative_heading_does_not_match_normative_substring(self):
        html = '''<html><body>
        <h3 id="normative">Normative References</h3>
        <dl><dt id="biblio-required">[REQUIRED]</dt><dd>A normative reference.</dd></dl>
        <h3 id="informative">Non-Normative References</h3>
        <dl><dt id="biblio-background">[BACKGROUND]</dt><dd>Background only.</dd></dl>
        <h3>Normative References</h3>
        <dl><dt id="biblio-required-again">[REQUIRED-AGAIN]</dt><dd>Normative again.</dd></dl>
        </body></html>'''
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "original").mkdir()
            (root / "original/css.html").write_text(html)
            manifest = {"assets": [{"id": "css", "path": "original/css.html",
                                    "sha256": assets.sha(html.encode()), "contentType": "text/html",
                                    "finalUrl": "https://example.test/fixed/"}]}
            refs = assets.source_inventory(root, manifest)["directReferences"]
        self.assertEqual({r["entry"]: r["kind"] for r in refs}, {
            "biblio-required": "normative", "biblio-background": "informative",
            "biblio-required-again": "normative",
        })

    def test_actual_snapshot_non_normative_citations_remain_informative(self):
        inventory = json.loads(Path("docs/specs/epub-3.3/inventory.json").read_text())
        refs = {r["entry"]: r for r in inventory["directReferences"] if r["document"] == "css"}
        # Actual entries under Non-Normative References in the frozen CSS Snapshot.
        for name in ("biblio-css-align-3", "biblio-css-animations-1", "biblio-css-break-3",
                     "biblio-css-cascade-3", "biblio-css-cascade-5", "biblio-css-color-3",
                     "biblio-css-color-adjust-1", "biblio-css-conditional-4"):
            with self.subTest(reference=name):
                self.assertEqual(refs[name]["kind"], "informative")


if __name__ == "__main__":
    unittest.main(verbosity=2)
