"""Independent spot checks against fixed EPUB 3.3 REC 2026-01-13.

These are not a substitute for all-section semantic review. Run from the
repository root. The source quotes and expectations were selected from the
original REC before receiving the semantic matrix implementation.
"""
import hashlib
import json
from pathlib import Path
import unittest


ROOT = Path("docs/specs/epub-3.3")
EPUB_SHA = "f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99"


class ParentSemanticAcceptance(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        actual = hashlib.sha256((ROOT / "original/epub.html").read_bytes()).hexdigest()
        if actual != EPUB_SHA:
            raise AssertionError("Spot checks require the reviewed fixed REC, not a later edition")
        cls.matrix = json.loads((ROOT / "matrix.json").read_text())
        cls.rows = cls.matrix["rows"]

    def manual_rows(self, section):
        ids = {c["featureId"] for c in self.matrix["manualConstraints"]}
        return [r for r in self.rows if r["featureId"] in ids
                and r["document"] == "epub" and r["specSection"] == section]

    def assert_mapped(self, row):
        self.assertEqual(row["decision"], "mapped")
        self.assertTrue(row["reason"])
        self.assertNotIn(row["applicability"], (None, "", "not-reviewed"))
        self.assertNotIn(row["phase"], (None, "", "not-assigned"))

    def test_manifest_required_attributes_are_individual_constraints(self):
        # These labels are not marked rfc2119 and are not usage/cardinality slots.
        # A single broad Attributes row must not substitute for all four rules.
        rows = self.manual_rows("sec-item-elem")
        found_ids = set()
        for quote in ("fallback [conditionally required]", "href [required]",
                      "id [required]", "media-type [required]"):
            with self.subTest(quote=quote):
                matches = [r for r in rows if quote in r["excerpt"]]
                self.assertEqual(len(matches), 1, f"Missing independent attribute constraint: {quote}")
                self.assert_mapped(matches[0])
                found_ids.add(matches[0]["featureId"])
        self.assertEqual(len(found_ids), 4)

    def test_linear_default_has_own_unmarked_constraint(self):
        # The following MUST about at least one linear item is a different rule.
        quote = ('A linear itemref element is one whose linear attribute value is '
                 'explicitly set to "yes" or that omits the attribute')
        rows = [r for r in self.manual_rows("sec-itemref-elem") if quote in r["excerpt"]]
        self.assertTrue(rows, "Missing the unmarked default-yes definition")
        for row in rows:
            self.assert_mapped(row)

    def test_landmarks_optional_and_maximum_have_distinct_instances(self):
        quote = ("The landmarks nav element is OPTIONAL in EPUB navigation documents "
                 "and MUST NOT occur more than once.")
        rows = [r for r in self.rows if r["document"] == "epub"
                and r["specSection"] == "sec-nav-landmarks"
                and r["kind"] == "bcp14" and r["excerpt"] == quote]
        self.assertEqual(len(rows), 2)
        self.assertEqual({r["normativeLevel"] for r in rows}, {"OPTIONAL", "MUST NOT"})
        self.assertEqual(len({r["featureId"] for r in rows}), 2)
        for row in rows:
            self.assert_mapped(row)


if __name__ == "__main__":
    unittest.main(verbosity=2)
