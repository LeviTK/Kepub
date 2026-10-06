"""Independent R5 source checks; no full-prose completeness claim.

Statements and their conditional meaning were read from the fixed REC before
examining the reconciliation selectors. Lowercase advice is not BCP14 MUST.
Run from the repository under review; the original source hashes are pinned.
"""
import copy
import hashlib
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


STATEMENTS = (
    ("epub", "sec-opf-dccontributor", "p[2]", False,
     "The requirements for the dc:contributor element are identical to those for the dc:creator element in all other respects."),
    ("epub", "sec-opf-dcidentifier", "p[3]", True,
     "EPUB creators should not issue new identifiers when making minor revisions"),
    ("epub", "sec-opf-dcdate", "p[3]", True,
     "should express additional dates using the specialized date properties"),
    ("epub", "sec-opf-dcsubject", "p[1]", True,
     "but may use a code value if the subject taxonomy does not provide a separate descriptive label"),
    ("epub", "sec-metadata-last-modified", "p[3]", True,
     "should update the last modified date whenever they make changes"),
    ("epub", "sec-core-media-types", "ul[1]/li[1]/p[2]", True,
     "If the table lists more than one media type, the first one is the preferred media type."),
    ("epub", "security-privacy-recommendations", "p[7]", True,
     "When publishers and vendors must use digital rights management schemes, they should prefer schemes"),
    ("rs", "sec-css", "p[4]", True,
     "publicly document their user agent style sheets and how they interact"),
)


class R5SourceChecks(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.nodes, cls.anchors = {}, {}
        for doc, expected in (
            ("epub", "f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99"),
            ("rs", "2e8d4400d1cce9080e729df8292d8795b4d76cb1a20be2387a263bcfbcd7ca9e"),
        ):
            raw = (assets.ROOT / f"original/{doc}.html").read_bytes()
            assert hashlib.sha256(raw).hexdigest() == expected
            root = assets.DOM(raw.decode()).root
            cls.nodes[doc] = {n.path(): n for n in root.walk()}
            cls.anchors[doc] = {n.attrs["id"]: n for n in root.walk() if n.attrs.get("id")}

    def test_valid_matrix_and_existing_advisory_positive(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        path = self.anchors["epub"]["sec-opf-dctitle"].path() + "/p[3]"
        rows = [r for r in self.matrix["rows"] if r["document"] == "epub" and r["domPath"] == path]
        self.assertTrue(any(r["decision"] == "mapped" and "ADVISORY" in r["normativeLevel"] for r in rows))

    def test_inheritance_and_conditional_advisories_have_direct_source_records(self):
        for doc, anchor, suffix, advisory, quote in STATEMENTS:
            with self.subTest(doc=doc, anchor=anchor, suffix=suffix):
                path = self.anchors[doc][anchor].path() + "/" + suffix
                node = self.nodes[doc][path]
                self.assertIn(quote, assets.normalized(node))
                self.assertFalse(assets.non_normative(node))
                rows = [r for r in self.matrix["rows"] if r["document"] == doc
                        and r["domPath"] == path and r["decision"] == "mapped"]
                self.assertTrue(rows, "Fixed-source statement has no direct mapped row")
                self.assertTrue(any(quote in r["excerpt"] for r in rows))
                if advisory:
                    self.assertTrue(any("ADVISORY" in r["normativeLevel"] for r in rows))
                    self.assertFalse(any(r["normativeLevel"] in ("MUST", "SHALL", "REQUIRED", "SHOULD")
                                         for r in rows), "Lowercase advice was promoted to a BCP14 requirement")

    def test_completed_source_families_cannot_silently_lose_their_record(self):
        for doc, anchor, suffix in (
            ("epub", "sec-opf-dccontributor", "p[2]"),
            ("epub", "sec-core-media-types", "ul[1]/li[1]/p[2]"),
            ("rs", "sec-css", "p[4]"),
        ):
            with self.subTest(doc=doc, anchor=anchor):
                changed = copy.deepcopy(self.matrix)
                path = self.anchors[doc][anchor].path() + "/" + suffix
                for key in ("rows", "manualConstraints"):
                    changed[key] = [r for r in changed[key] if not (r["document"] == doc
                                    and (r["domPath"] == path or r["domPath"].startswith(path + "/")))]
                with self.assertRaisesRegex(ValueError, "source member"):
                    assets.verify_mapping(assets.ROOT, matrix=changed)


if __name__ == "__main__":
    unittest.main(verbosity=2)
