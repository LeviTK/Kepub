"""Independent fixed-REC definitions and scope checks following Droid S0 R3.

Expected statements were read from the immutable REC, not from the candidate
inventory or the implementation's reconciliation selector list. These cases
check specific omissions; they are not a proof of all prose completeness.
"""
import copy
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


STATEMENTS = (
    ("sec-exempt-resources", "p[4]", "a resource is classified as an exempt resource if:"),
    ("sec-exempt-resources", "ul[1]/li[1]", "not referenced from a spine itemref"),
    ("sec-exempt-resources", "ul[1]/li[2]", "not embedded directly"),
    ("sec-xhtml-svg", "ul[1]/li[1]", "by reference"),
    ("sec-xhtml-svg", "ul[1]/li[2]", "same content conformance constraints"),
    ("sec-svg-restrictions", "p[1]", "SVG embedded by inclusion in XHTML"),
    ("sec-link-elem", "p[3]", "publication resources only when"),
    ("sec-link-elem", "ul[1]/li[1]/p[1]", "referenced from the spine; or"),
    ("sec-link-elem", "ul[1]/li[2]/p[1]", "included or embedded"),
    ("sec-metadata-values", "p[1]", "mandatory child text content"),
    ("sec-metadata-values", "p[3]", "collapsed to a single space"),
    ("sec-opf-dctitle", "p[2]", "first dc:title element in document order"),
    ("sec-opf-dccreator", "p[5]", "determines the display priority"),
    ("sec-scripted-support", "p[3]", "does not represent scripted content"),
    ("sec-scripted-context", "p[1]", "two contexts for script execution"),
    ("sec-scripted-context", "ul[1]/li[1]", "container constrained"),
    ("sec-scripted-context", "ul[1]/li[2]", "spine level"),
    ("sec-scripted-context", "p[2]", "makes no difference to its executing context"),
    ("sec-scripted-container-constrained", "p[1]", "either of the following"),
    ("sec-scripted-container-constrained", "ul[1]/li[1]", "XHTML content document"),
    ("sec-scripted-container-constrained", "ul[1]/li[2]", "SVG content document"),
    ("sec-scripted-spine", "p[1]", "top-level content document"),
    ("sec-meta-elem", "ul[1]/li[1]", "omits a refines attribute"),
    ("sec-meta-elem", "ul[1]/li[2]", "associated with another expression or resource"),
    ("sec-itemref-elem", "p[4]", "primary reading order"),
    ("sec-itemref-elem", "p[6]", "only a hint"),
    ("sec-opf-dcsubject", "p[5]", "case sensitive only when the designated scheme requires"),
    ("sec-xhtml-req", "p[2]", "Unless specified otherwise"),
    ("sec-container-filenames", "p[1]", "scalar value strings"),
    ("sec-container-iri", "p[2]", "content URL of a file or directory"),
    ("sec-container-iri", "p[3]", "path-relative-scheme-less-url string"),
    ("sec-property-datatype", "p[4]", "default vocabulary for that attribute"),
    ("sec-prefix-attr", "p[4]", "not namespaced when used in the package document"),
    ("sec-role", "table[1]/tbody[1]/tr[2]/td[1]/p[3]", "importance of the roles should match"),
    ("app-media-type-app-oebps-package", "dl[1]/dd[5]/p[1]", "8bit if UTF-8; binary if UTF-16"),
    ("app-media-type-app-oebps-package", "dl[1]/dd[5]/p[2]", "binary content-transfer-encoding must be used"),
    ("app-media-type-app-oebps-package", "dl[1]/dd[6]/p[1]", "well-formed XML"),
    ("app-media-type-app-oebps-package", "dl[1]/dd[6]/p[3]", "should rigorously check the size and validity"),
    ("app-media-type", "dl[1]/dd[5]/p[1]", "binary files encoded in the application/zip media type"),
    ("app-media-type", "dl[1]/dd[6]/p[1]", "should rigorously check the size and validity"),
    ("app-media-type", "dl[1]/dd[6]/p[3]", "apply to OCF ZIP container files"),
)


class R3Definitions(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        root = assets.DOM((assets.ROOT / "original/epub.html").read_text()).root
        cls.nodes = {n.path(): n for n in root.walk()}
        cls.anchors = {n.attrs["id"]: n for n in root.walk() if n.attrs.get("id")}

    def test_existing_matrix_is_valid_control(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)

    def test_independent_definitions_are_mapped_in_their_actual_scope(self):
        for anchor, suffix, quote in STATEMENTS:
            path = self.anchors[anchor].path() + "/" + suffix
            with self.subTest(anchor=anchor, suffix=suffix):
                node = self.nodes[path]
                self.assertIn(quote, assets.normalized(node))
                self.assertFalse(assets.non_normative(node))
                paths = {path}
                # A single-paragraph list item may be represented by its li.
                # A multi-paragraph dd/section cannot substitute for a clause.
                if node.tag == "p" and node.parent.tag == "li":
                    paths.add(node.parent.path())
                matching = [r for r in self.matrix["rows"] if r["document"] == "epub"
                            and r["domPath"] in paths and r["decision"] == "mapped"]
                self.assertTrue(matching, "Definition missing despite completed section review")
                self.assertTrue(any(quote in r["excerpt"] for r in matching))

    def test_whole_definition_family_cannot_vanish_while_section_is_complete(self):
        for anchor, prefixes in (
            ("sec-exempt-resources", ("p[4]", "ul[1]")),
            ("sec-xhtml-svg", ("ul[1]",)),
            ("sec-metadata-values", ("p[1]", "p[3]")),
            ("sec-scripted-context", ("p[1]", "p[2]", "ul[1]")),
        ):
            changed = copy.deepcopy(self.matrix)
            paths = [self.anchors[anchor].path() + "/" + p for p in prefixes]
            for key in ("rows", "manualConstraints"):
                changed[key] = [r for r in changed[key] if not (r["document"] == "epub"
                                and any(r["domPath"] == p or r["domPath"].startswith(p + "/")
                                        for p in paths))]
            with self.subTest(anchor=anchor), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_near_duplicate_informative_html_overview_is_not_a_new_requirement(self):
        overview = self.anchors["sec-overview-relations-html"].path() + "/p[3]"
        normative = self.anchors["sec-xhtml-req"].path() + "/p[2]"
        self.assertIn("inherits all definitions", assets.normalized(self.nodes[overview]))
        self.assertIn("inherit all definitions", assets.normalized(self.nodes[normative]))
        self.assertTrue(assets.non_normative(self.nodes[overview]))
        self.assertFalse(assets.non_normative(self.nodes[normative]))
        self.assertFalse(any(r["document"] == "epub" and r["domPath"] == overview
                             and r["decision"] == "mapped" for r in self.matrix["rows"]))


if __name__ == "__main__":
    unittest.main(verbosity=2)
