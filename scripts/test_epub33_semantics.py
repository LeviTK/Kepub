"""Source-derived semantic index counterexamples; no EPUB behavior is executed."""
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import epub33_semantics as semantics
from epub33_assets import ROOT, write_json


class SemanticEvidence(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.index = semantics.build(ROOT)

    def mutated_review(self, name, mutate):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        for path in ROOT.iterdir():
            if path.name == "reviews":
                shutil.copytree(path, root / path.name)
            else:
                (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
        path = root / "reviews" / name
        review = json.loads(path.read_text())
        mutate(review)
        write_json(path, review)
        return root

    def test_center_alias_keeps_distinct_levels(self):
        case = next(r for r in self.index["cases"] if r["id"] == "fxl-page-spread-center")
        texts = [r for link in case["fixedSourceCorrespondences"] for r in link["clauses"]]
        self.assertTrue(any("alias of the spread-none" in r["excerpt"] for r in texts))
        self.assertTrue(any(r["normativeLevel"] == "MUST NOT" and "synthetic spread" in r["excerpt"] for r in texts))
        self.assertTrue(any(r["normativeLevel"] == "SHOULD" and "center of the screen" in r["excerpt"] for r in texts))
        self.assertFalse(case["executed"])

    def test_obsolete_report_url_is_not_silently_rewritten(self):
        case = next(r for r in self.index["cases"] if r["id"] == "pub-cmt-mp4")
        self.assertTrue(any(url.endswith("#cmt-cmt-mp4-aac") for url in case["reportIdentity"]["references"]))
        self.assertTrue(any("audio/mp4" in link["excerpt"] for link in case["fixedSourceCorrespondences"]))
        self.assertTrue(case["explicitCorrespondenceReview"]["applicability"])
        self.assertFalse(self.index["semanticComplete"])

    def test_omitted_css_module_rejected_from_actual_tier_list(self):
        root = self.mutated_review("supporting.json", lambda r: r["cssModuleEntries"].pop())
        with self.assertRaisesRegex(ValueError, "CSS-tier-drift"):
            semantics.build(root)

    def test_omitted_supporting_section_rejected(self):
        root = self.mutated_review("supporting.json", lambda r: r["sectionReviews"].pop(0))
        with self.assertRaisesRegex(ValueError, "supporting section review"):
            semantics.build(root)

    def test_fabricated_supporting_quote_rejected(self):
        root = self.mutated_review("supporting.json", lambda r: r["sectionReviews"][0].update(excerpt="Fabricated source quote"))
        with self.assertRaisesRegex(ValueError, "excerpt drift"):
            semantics.build(root)

    def test_case_execution_claim_rejected(self):
        root = self.mutated_review("official-cases.json", lambda r: r["cases"][0].update(executed=True))
        with self.assertRaisesRegex(ValueError, "invented execution"):
            semantics.build(root)

    def test_css_module_source_inventory_cannot_claim_render_support(self):
        root = self.mutated_review("supporting.json", lambda r: r["cssModuleEntries"][0]["capabilities"].update(render="supported"))
        with self.assertRaisesRegex(ValueError, "CSS module source review must not invent"):
            semantics.build(root)

    def test_supporting_source_counts_cannot_replace_stage_review(self):
        root = self.mutated_review("supporting.json", lambda r: r["sectionReviews"][0].update(plannedPhase=[]))
        with self.assertRaisesRegex(ValueError, "supporting section lacks"):
            semantics.build(root)

    def test_obsolete_anchor_requires_explicit_semantic_review(self):
        root = self.mutated_review("official-anchor-amendments.json", lambda r: r["mappings"].pop(0))
        with self.assertRaisesRegex(ValueError, "obsolete report anchor"):
            semantics.build(root)


if __name__ == "__main__":
    unittest.main()
