"""Bounded source-family controls, not automatic prose completeness proof."""
import copy
import json
from pathlib import Path
import tempfile
import unittest

import epub33_assets as assets
from test_epub33_parent_r5_inheritance import STATEMENTS


class R5SourceMembers(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.nodes = {
            doc: {n.path(): n for n in assets.DOM(
                (assets.ROOT / f"original/{doc}.html").read_text()).root.walk()}
            for doc in ("epub", "rs", "a11y")}
        cls.anchors = {doc: {n.attrs["id"]: n for n in nodes.values()
                            if n.attrs.get("id")} for doc, nodes in cls.nodes.items()}

    def reconcile(self, rows, reviews=None):
        assets.reconcile_source_members(self.nodes, {r["featureId"]: r for r in rows},
                                        self.matrix["sectionReviews"] if reviews is None else reviews)

    def test_each_member_requires_its_own_source_not_an_ancestor_or_sibling(self):
        self.reconcile(self.matrix["rows"])
        for doc, anchor, suffix, _, _ in STATEMENTS:
            path = self.anchors[doc][anchor].path() + "/" + suffix
            original = next(r for r in self.matrix["rows"] if
                            r["document"] == doc and r["domPath"] == path)
            remaining = [r for r in self.matrix["rows"] if r is not original]
            sibling = "p[2]" if suffix == "p[1]" else "p[1]"
            for replacement in (None, self.anchors[doc][anchor].path(),
                                self.anchors[doc][anchor].path() + "/" + sibling):
                # Even giving a wrong-path row the full missing statement must
                # not satisfy reconciliation. A duplicate sibling is not a member.
                changed = copy.deepcopy(remaining)
                if replacement is not None:
                    changed.append({**original, "featureId": "wrong-path-control",
                                    "domPath": replacement})
                with self.subTest(doc=doc, anchor=anchor, replacement=replacement):
                    with self.assertRaisesRegex(ValueError, "source member"):
                        self.reconcile(changed)
            pending = copy.deepcopy(self.matrix["sectionReviews"])
            next(s for s in pending if s["document"] == doc and
                 s["anchor"] == anchor)["status"] = "pending"
            self.reconcile(remaining, pending)  # Truthfully incomplete is allowed, not approved.

    def test_inheritance_boundary_and_no_behavior_promotion(self):
        section = self.anchors["epub"]["sec-opf-dccontributor"]
        self.assertIn("played a secondary role", assets.normalized(
            self.nodes["epub"][section.path() + "/p[1]"]))
        row = next(r for r in self.matrix["rows"] if r["document"] == "epub" and
                   r["domPath"] == section.path() + "/p[2]")
        self.assertEqual(row["normativeLevel"], "UNMARKED INHERITANCE DEFINITION")
        self.assertIn("in all other respects", row["excerpt"])
        self.assertIn("secondary role", row["reason"])
        for doc, anchor, suffix, _, _ in STATEMENTS:
            path = self.anchors[doc][anchor].path() + "/" + suffix
            row = next(r for r in self.matrix["rows"] if r["document"] == doc and r["domPath"] == path)
            for dimension in ("preserve", "parse", "edit", "render", "validate"):
                self.assertEqual(row[dimension], "not-tested")
            self.assertEqual(row["evidence"], [])
            self.assertEqual(row["testIds"], [])
        css = next(r for r in self.matrix["rows"] if r["document"] == "rs" and
                   r["domPath"] == self.anchors["rs"]["sec-css"].path() + "/p[4]")
        self.assertNotIn("T1", css["phase"])

    def test_failed_multi_amendment_import_does_not_partially_write(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for path in assets.ROOT.iterdir():
                if path.name == "matrix.json":
                    (root / path.name).write_bytes(path.read_bytes())
                else:
                    (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
            good = root / "good.json"
            packet = json.loads((assets.ROOT / "reviews/r5-publication-amendments.json").read_text())
            note = "Atomicity positive control: changed contributor inheritance review note."
            packet["sectionNotes"]["sec-opf-dccontributor"] = note
            assets.write_json(good, packet)
            bad = root / "bad.json"
            assets.write_json(bad, {"document": "rs", "sourceHash": "wrong",
                                    "constraints": []})
            before = (root / "matrix.json").read_bytes()
            assets.import_reviews(root, [], [good])
            self.assertNotEqual((root / "matrix.json").read_bytes(), before)
            expected = json.loads(before)
            next(s for s in expected["sectionReviews"] if s["document"] == "epub" and
                 s["anchor"] == "sec-opf-dccontributor")["notes"] = note
            self.assertEqual(json.loads((root / "matrix.json").read_text()), expected)
            (root / "matrix.json").write_bytes(before)
            with self.assertRaisesRegex(ValueError, "amendment source identity"):
                assets.import_reviews(root, [], [good, bad])
            self.assertEqual((root / "matrix.json").read_bytes(), before)


if __name__ == "__main__":
    unittest.main(verbosity=2)
