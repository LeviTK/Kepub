"""Bounded R3 definition reconciliation; no general natural-language classifier."""
import copy
import json
from pathlib import Path
import tempfile
import unittest

import epub33_assets as assets


FOLLOWUP = (
    ("html-script-element", "p[3]", "exempt from fallback requirements", "UNMARKED EXEMPTION DEFINITION"),
    ("sec-opf-dctitle", "p[3]", "should use only a single dc:title", "UNMARKED ADVISORY"),
    ("sec-opf-dccreator", "p[2]", "should contain the name of the creator", "UNMARKED ADVISORY"),
    ("sec-opf-dccreator", "p[4]", "should specify each in a separate dc:creator", "UNMARKED CONDITIONAL ADVISORY"),
    ("sec-opf-dccreator", "p[6]", "should represent secondary contributors", "UNMARKED ADVISORY"),
    ("sec-meta-elem", "p[2]", "text content of the element represents the assertion", "UNMARKED METADATA DEFINITION"),
    ("sec-meta-elem", "p[5]", "Meta Properties Vocabulary is the default", "UNMARKED DEFAULT VOCABULARY"),
    ("sec-link-elem", "p[10]", "Metadata Link Vocabulary is the default", "UNMARKED DEFAULT VOCABULARY"),
    ("sec-item-resource-properties", "p[2]", "Manifest Properties Vocabulary is the default", "UNMARKED DEFAULT VOCABULARY"),
    ("sec-itemref-elem", "p[9]", "Spine Properties Vocabulary is the default", "UNMARKED DEFAULT VOCABULARY"),
)


class R3SourceMembers(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        dom = assets.DOM((assets.ROOT / "original/epub.html").read_text()).root
        cls.sections = {n.attrs["id"]: n for n in dom.walk()
                        if n.tag == "section" and n.attrs.get("id")}

    def omit(self, anchor, suffix):
        changed = copy.deepcopy(self.matrix)
        path = self.sections[anchor].path() + "/" + suffix
        for key in ("rows", "manualConstraints"):
            changed[key] = [r for r in changed[key] if not
                            (r["document"] == "epub" and
                             (r["domPath"] == path or r["domPath"].startswith(path + "/")))]
        return changed

    def test_general_exemption_and_link_resource_families_are_independent(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        for anchor in ("sec-exempt-resources", "sec-link-elem"):
            with self.subTest(anchor=anchor), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=self.omit(anchor, "ul[1]"))

    def test_embedded_svg_and_normative_html_inheritance_cannot_disappear(self):
        for anchor, suffix in (("sec-xhtml-svg", "ul[1]"),
                               ("sec-svg-restrictions", "p[1]"),
                               ("sec-xhtml-req", "p[2]")):
            with self.subTest(anchor=anchor), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=self.omit(anchor, suffix))

    def test_mime_encoding_and_each_security_statement_are_separate(self):
        for anchor, suffix in (("app-media-type-app-oebps-package", "dl[1]/dd[5]"),
                               ("app-media-type-app-oebps-package", "dl[1]/dd[6]/p[3]"),
                               ("app-media-type", "dl[1]/dd[6]/p[3]")):
            with self.subTest(anchor=anchor, suffix=suffix), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=self.omit(anchor, suffix))

    def test_parent_followup_actual_text_scope_and_advisory_levels(self):
        nodes = {n.path(): n for section in self.sections.values() for n in section.walk()}
        for anchor, suffix, quote, level in FOLLOWUP:
            path = self.sections[anchor].path() + "/" + suffix
            with self.subTest(anchor=anchor, suffix=suffix):
                self.assertIn(quote, assets.normalized(nodes[path]))
                self.assertFalse(assets.non_normative(nodes[path]))
                rows = [r for r in self.matrix["rows"] if r["document"] == "epub" and r["domPath"] == path]
                self.assertEqual(len(rows), 1)
                self.assertEqual(rows[0]["decision"], "mapped")
                self.assertEqual(rows[0]["normativeLevel"], level)
                self.assertIn(quote, rows[0]["excerpt"])
                for dimension in ("preserve", "parse", "edit", "render", "validate"):
                    self.assertEqual(rows[0][dimension], "not-tested")

    def test_each_followup_member_cannot_be_replaced_by_neighboring_defaults(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        for anchor, suffix, _, _ in FOLLOWUP:
            with self.subTest(anchor=anchor, suffix=suffix), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=self.omit(anchor, suffix))

    def test_section_notes_are_source_bound_atomic_and_idempotent(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for path in assets.ROOT.iterdir():
                if path.name == "matrix.json":
                    (root / path.name).write_bytes(path.read_bytes())
                else:
                    (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
            amendment = root / "note-amendment.json"
            packet = {"document": "epub", "sourceHash": assets.sha((root / "original/epub.html").read_bytes()),
                      "constraints": [], "sectionNotes": {}}
            before = (root / "matrix.json").read_bytes()
            for notes in ({"not-a-source-section": "reviewed"}, {"sec-meta-elem": " "},
                          {"sec-meta-elem": 42}):
                assets.write_json(amendment, {**packet, "sectionNotes": notes})
                with self.subTest(notes=notes), self.assertRaisesRegex(ValueError, "section note source identity"):
                    assets.import_reviews(root, [], [amendment])
                self.assertEqual((root / "matrix.json").read_bytes(), before)
            note = "Bounded source review: meta property statement and text assertion; not independent approval."
            assets.write_json(amendment, {**packet, "sectionNotes": {"sec-meta-elem": note}})
            assets.import_reviews(root, [], [amendment])
            changed = json.loads((root / "matrix.json").read_text())
            expected = copy.deepcopy(self.matrix)
            record = next(r for r in expected["sectionReviews"] if r["document"] == "epub" and
                          r["anchor"] == "sec-meta-elem")
            record.update(notes=note, reviewer="coding Orb source/context correction; not independent full review")
            self.assertEqual(changed, expected)
            first = (root / "matrix.json").read_bytes()
            assets.import_reviews(root, [], [amendment])
            self.assertEqual((root / "matrix.json").read_bytes(), first)


if __name__ == "__main__":
    unittest.main(verbosity=2)
