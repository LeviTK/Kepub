"""Definitions are normative; named Explanation blocks and examples are not.

Independent expectation: fixed REC §1.5 and official epub-specs commits
642a45d0cd81df784ff852d32a55a0f28f87d083 and
271f0d7f7c05652ee0fcf77fafe6ac89dc9b08e4. The former introduced the
conformance sentence while changing the named URL Explanation blocks, not
the three font algorithm definition paragraphs.
"""
import copy
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


class AlgorithmScopeCorrection(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.entries = {e["id"]: e for e in json.loads((assets.ROOT / "manifest.json").read_text())["assets"]}
        cls.nodes = {doc: list(assets.DOM((assets.ROOT / f"original/{doc}.html").read_text()).root.walk())
                     for doc in ("epub", "rs")}
        cls.base = "/html[1]/body[1]/section[6]/section[4]/section[4]"

    def test_valid_matrix_and_existing_illustrative_pseudocode_control(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        rows = [r for r in self.matrix["rows"] if r["document"] == "epub"
                and r["domPath"].startswith(self.base + "/ol[1]/")]
        self.assertTrue(rows)
        self.assertTrue(all(r["decision"] == "excluded" for r in rows))

    def test_all_three_font_definition_paragraphs_are_mapped(self):
        for number, expected in ((1, "first 1040 bytes"), (2, "logical exclusive or (XOR)"), (3, "21st byte")):
            path = f"{self.base}/p[{number}]"
            node = next(n for n in self.nodes["epub"] if n.path() == path)
            self.assertIn(expected, assets.normalized(node))
            with self.subTest(paragraph=number):
                rows = [r for r in self.matrix["rows"] if r["document"] == "epub"
                        and r["domPath"] == path and r["decision"] == "mapped"]
                self.assertTrue(rows, "Unmarked algorithm definition must not be treated as an Explanation/example")

    def test_restoring_existing_definition_identities_is_not_forbidden(self):
        changed = copy.deepcopy(self.matrix)
        for number in (1, 3):
            row = next(r for r in changed["rows"] if r["document"] == "epub"
                       and r["domPath"] == f"{self.base}/p[{number}]")
            row.update(decision="mapped", phase="T3/T5",
                       applicability="Preserved font obfuscation algorithm definition",
                       reason="Normative definition outside the explicitly exemplary pseudo-code and named explanations")
        assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_named_explanation_cannot_be_promoted_to_normative(self):
        for document in ("epub", "rs"):
            details = next(n for n in self.nodes[document] if n.tag == "details"
                           and any(c.tag == "summary" and assets.normalized(c) == "Explanation" for c in n.walk()))
            node = next(n for n in details.walk() if n.tag == "p")
            clause = assets.candidate(self.entries[document], node, node, "manual-normative", 1, "UNMARKED NORMATIVE")
            changed = copy.deepcopy(self.matrix)
            self.assertNotIn(clause["featureId"], {r["featureId"] for r in changed["rows"]})
            changed["manualConstraints"].append(clause)
            changed["rows"].append({**clause, "decision": "mapped", "reason": "Incorrectly claimed normative explanation",
                                    "applicability": "Algorithm behavior", "phase": "T2", "platform": "not-tested",
                                    "gap": "No implementation claimed", "testIds": [], "evidence": [],
                                    **{k: "not-tested" for k in ("preserve", "parse", "edit", "render", "validate")}})
            with self.subTest(document=document), self.assertRaisesRegex(ValueError, "non-normative source scope"):
                assets.verify_mapping(assets.ROOT, matrix=changed)


if __name__ == "__main__":
    unittest.main(verbosity=2)
