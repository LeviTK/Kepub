"""Fixed-source checks for inherited constraints and REC test backlinks."""
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


class ListAndBacklinkAcceptance(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.semantic = json.loads((assets.ROOT / "semantic-index.json").read_text())
        cls.nodes = {}
        for doc in ("epub", "rs", "a11y"):
            root = assets.DOM((assets.ROOT / f"original/{doc}.html").read_text()).root
            cls.nodes[doc] = {n.path(): n for n in root.walk()}

    def test_independently_decidable_list_leaves_have_mappings(self):
        # Paths and cardinalities were read from the fixed RECs, not obtained
        # from the candidate inventory or the code generating the matrix.
        lists = (
            ("epub", "/section[6]/section[2]/section[3]/ul[1]/li[3]/ul[1]", 20),
            ("epub", "/section[5]/section[7]/ul[1]", 4),
            ("epub", "/section[6]/section[2]/section[6]/section[3]/section[2]/section[1]/ul[1]", 8),
            ("epub", "/section[7]/section[6]/section[2]/section[1]/ul[1]", 5),
            ("epub", "/section[9]/section[4]/section[2]/ul[1]", 2),
            ("epub", "/section[8]/section[1]/section[3]/section[5]/ul[1]", 2),
            ("epub", "/section[5]/section[6]/ul[1]", 4),
            ("a11y", "/section[5]/section[4]/section[2]/section[3]/section[2]/dl[1]/dd[3]/ul[1]", 2),
        )
        for doc, suffix, count in lists:
            container = self.nodes[doc]["/html[1]/body[1]" + suffix]
            leaves = [n for n in container.walk() if n.tag == "li"
                      and not any(c.tag == "li" for c in list(n.walk())[1:])]
            self.assertEqual(len(leaves), count, suffix)
            for leaf in leaves:
                with self.subTest(document=doc, text=assets.normalized(leaf)):
                    matching = [r for r in self.matrix["rows"] if r["document"] == doc
                                and (r["domPath"] == leaf.path()
                                     or r["domPath"].startswith(leaf.path() + "/"))]
                    self.assertTrue(matching, "An introductory colon does not register the leaf constraint")
                    self.assertTrue(any(r["decision"] == "mapped" for r in matching))

    def test_eleven_missing_rec_contexts_are_present_without_losing_report_target(self):
        expected = (
            ("cnt-svg-support", "epub", "/section[5]/section[2]/table[1]/tbody[1]/tr[5]"),
            ("cnt-xhtml-support", "rs", "/section[5]/p[1]"),
            ("ocf-package_arbitrary", "rs", "/section[6]/p[1]"),
            ("pkg-dir-auto_root-rtl", "rs", "/section[7]/section[1]/p[1]"),
            ("pkg-spine-nonlinear-activation", "rs", "/section[7]/section[5]/p[2]"),
            ("fxl-spine-overrides_behave-as-global", "rs", "/section[7]/section[5]/section[1]/p[3]"),
            ("nav-spine_in-spine", "rs", "/section[9]/p[1]"),
            ("lay-rendition-flow-pre-pag", "rs", "/section[10]/section[1]/section[1]/section[4]/p[3]"),
            ("mol-support_xhtml", "rs", "/section[11]/section[1]/p[2]"),
            ("mol-support_xhtml-fxl", "rs", "/section[11]/section[1]/p[2]"),
            ("scr-readingsystem-features", "rs", "/section[19]/section[4]/section[1]/section[2]/p[1]"),
        )
        cases = {c["id"]: c for c in self.semantic["cases"]}
        self.assertEqual(len(cases), 169)
        for case, doc, suffix in expected:
            path = "/html[1]/body[1]" + suffix
            node = self.nodes[doc][path]
            source_refs = [v.strip() for n in node.walk()
                           for v in n.attrs.get("data-tests", "").split(",")]
            self.assertIn("#" + case, source_refs)
            with self.subTest(case=case, source=path):
                self.assertTrue(any(c["document"] == doc and c["sourceDOM"] == path
                                    for c in cases[case]["fixedSourceCorrespondences"]))
        # The report's publication-side condition and the RS backlink are
        # complementary; adding the latter must not replace the former.
        original = cases["pkg-spine-nonlinear-activation"]
        self.assertTrue(any(c["document"] == "epub"
                            and "provide a means of accessing all non-linear content" in c["excerpt"]
                            for c in original["fixedSourceCorrespondences"]))
        self.assertFalse(original["executed"])
        self.assertEqual(original["result"], "not-tested")


if __name__ == "__main__":
    unittest.main(verbosity=2)
