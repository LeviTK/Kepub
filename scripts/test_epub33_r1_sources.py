"""R1 source-binding regressions beyond the unchanged parent controls."""
import copy
import json
import unittest

import epub33_assets as assets


class InheritedSourceBindings(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.nodes = {n.path(): n for n in assets.DOM((assets.ROOT / "original/epub.html").read_text()).root.walk()}

    def test_self_consistent_context_cannot_bind_unrelated_real_paragraph(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        changed = copy.deepcopy(self.matrix)
        clause = next(c for c in changed["manualConstraints"] if c["document"] == "epub" and c["domPath"] ==
                      "/html[1]/body[1]/section[5]/section[6]/ul[1]/li[1]")
        unrelated = self.nodes["/html[1]/body[1]/section[5]/section[6]/p[2]"]
        row = next(r for r in changed["rows"] if r["featureId"] == clause["featureId"])
        for target in (clause, row):
            target.update(contextDOM=unrelated.path(), contextExcerpt=assets.normalized(unrelated),
                          contextSHA256=assets.sha(assets.normalized(unrelated).encode()))
        with self.assertRaisesRegex(ValueError, "not the source list introduction/term"):
            assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_self_consistent_value_cannot_bind_other_real_term(self):
        changed = copy.deepcopy(self.matrix)
        clause = next(c for c in changed["manualConstraints"] if c["document"] == "epub" and c["domPath"] ==
                      "/html[1]/body[1]/section[10]/section[2]/section[2]/section[1]/dl[1]/dd[1]"
                      and "termDOM" in c)
        other = self.nodes["/html[1]/body[1]/section[10]/section[2]/section[2]/section[1]/dl[1]/dt[2]"]
        row = next(r for r in changed["rows"] if r["featureId"] == clause["featureId"])
        for target in (clause, row):
            target.update(termDOM=other.path(), termExcerpt=assets.normalized(other),
                          termSHA256=assets.sha(assets.normalized(other).encode()))
        with self.assertRaisesRegex(ValueError, "not the source list introduction/term"):
            assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_context_or_term_binding_cannot_be_deleted_from_both_records(self):
        for prefix in ("context", "term"):
            changed = copy.deepcopy(self.matrix)
            clause = next(c for c in changed["manualConstraints"] if c["kind"] == "manual-amendment"
                          and prefix + "DOM" in c)
            row = next(r for r in changed["rows"] if r["featureId"] == clause["featureId"])
            for target in (clause, row):
                for field in ("DOM", "Excerpt", "SHA256"):
                    target.pop(prefix + field)
            with self.subTest(prefix=prefix), self.assertRaisesRegex(ValueError, "missing required source binding"):
                assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_algorithm_explanation_paragraphs_cannot_be_repromoted(self):
        for suffix in ("p[1]", "p[3]"):
            changed = copy.deepcopy(self.matrix)
            row = next(r for r in changed["rows"] if r["featureId"] ==
                       f"epub:manual-normative:/html[1]/body[1]/section[6]/section[4]/section[4]/{suffix}:1")
            row.update(decision="mapped", phase="T3/T5", applicability="Claimed mandatory font operation",
                       reason="Pretend explanation is normative", gap="No implementation claim")
            with self.subTest(suffix=suffix), self.assertRaisesRegex(ValueError, "non-normative source scope"):
                assets.verify_mapping(assets.ROOT, matrix=changed)


if __name__ == "__main__":
    unittest.main(verbosity=2)
