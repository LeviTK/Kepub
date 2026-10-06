"""Independent fixed-REC controls for inherited non-normative source scope.

Run from the received repository root. These controls retain informative
source records while requiring the actual normative clauses to stay mapped.
"""
import hashlib
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


class InformativeScopeAcceptance(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path("docs/specs/epub-3.3")
        raw = (root / "original/epub.html").read_bytes()
        if hashlib.sha256(raw).hexdigest() != "f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99":
            raise AssertionError("Expected the fixed EPUB 3.3 REC 2026-01-13")
        cls.nodes = {n.path(): n for n in assets.DOM(raw.decode()).root.walk()}
        cls.rows = json.loads((root / "matrix.json").read_text())["rows"]

    def test_explicit_informative_sections_are_retained_as_exclusions(self):
        for section, minimum in (("sec-container-abstract-intro", 7),
                                 ("sec-docs-intro", 2),
                                 ("sec-nav-def-types-intro", 3)):
            rows = [r for r in self.rows if r["document"] == "epub" and r["specSection"] == section]
            with self.subTest(section=section):
                self.assertGreaterEqual(len(rows), minimum, "Keep source records rather than deleting the observations")
            for row in rows:
                with self.subTest(section=section, identity=row["featureId"]):
                    node = self.nodes[row["domPath"]]
                    self.assertTrue(any("informative" in n.attrs.get("class", "").split()
                                        for n in node.ancestors()))
                    self.assertEqual(row["decision"], "excluded",
                                     "REC §1.5 forbids promoting explicitly non-normative sections")
                    self.assertTrue(row["reason"])

    def test_actual_normative_constraints_remain_mapped(self):
        expectations = (
            ("sec-smil-text-elem", "REQUIRED", "As a REQUIRED child of the par element."),
            ("sec-nav-toc", "SHOULD", "EPUB creators SHOULD order the references in the toc nav element"),
        )
        for section, level, quote in expectations:
            with self.subTest(section=section):
                rows = [r for r in self.rows if r["document"] == "epub"
                        and r["specSection"] == section and r["kind"] == "bcp14"
                        and r["normativeLevel"] == level and quote in r["excerpt"]]
                self.assertEqual(len(rows), 1)
                self.assertEqual(rows[0]["decision"], "mapped")
                self.assertNotEqual(rows[0]["phase"], "S0-excluded")


if __name__ == "__main__":
    unittest.main(verbosity=2)
