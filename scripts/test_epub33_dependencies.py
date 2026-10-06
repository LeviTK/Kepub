"""Adversarial offline reference-review identity tests, not feature evidence."""
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import epub33_assets as assets
import epub33_semantics as semantics


class DependencyEvidence(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.entries = {e["id"]: e for e in assets.verify(assets.ROOT)["assets"]}
        cls.inventory = json.loads((assets.ROOT / "inventory.json").read_text())
        cls.nodes = {}
        for document, entry in cls.entries.items():
            if entry["path"].startswith("original/") and "html" in entry["contentType"]:
                dom = assets.DOM((assets.ROOT / entry["path"]).read_text()).root
                cls.nodes[document] = {n.path(): n for n in dom.walk()}

    def review(self, root=assets.ROOT):
        return semantics.dependency_review(root, self.entries, self.inventory, self.nodes)

    def mutation(self, path, change):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        shutil.copytree(assets.ROOT / "reviews", root / "reviews")
        shutil.copyfile(assets.ROOT / "external-dependencies.json", root / "external-dependencies.json")
        target = root / path
        data = json.loads(target.read_text())
        change(data)
        assets.write_json(target, data)
        return root

    def test_four_non_url_entries_retain_cited_edition_not_title_alias(self):
        records = {(r["document"], r["entry"]): r for r in self.review()["nonURLReferences"]}
        self.assertEqual(set(records), {("epub", "bib-us-ascii"), ("aria", "bib-dpub-aria"),
                                       ("aria", "bib-epub-3"), ("aria", "bib-wai-aria")})
        ascii_ref = records[("epub", "bib-us-ascii")]
        self.assertIn("ANSI X3.4, 1986", ascii_ref["citation"])
        self.assertEqual(ascii_ref["phases"], ["T1a", "T6"])
        for record in records.values():
            self.assertIsNone(record["url"])
            self.assertIsNone(record["sha256"])
            self.assertEqual(record["status"], "not-downloaded")
            self.assertEqual(set(record["capabilities"].values()), {"not-tested"})

    def test_explicit_revision_preserves_original_input_hash(self):
        review = self.review()
        self.assertEqual(review["historicalSourceInputSHA256"],
                         "6a2f061df2e22df6d9b14caba93534a4d38c7dec62547a3c80b6a6f4aa966090")
        self.assertEqual(review["historicalReviewSHA256"],
                         "75897ba88561782c39bc93600b57d7d900844157377bd1ff45e5106ac6a012e7")
        self.assertEqual(len(review["citationKindRevisions"]), 33)
        self.assertTrue(all(r["before"]["kind"] == "normative" and r["after"]["kind"] == "informative"
                            for r in review["citationKindRevisions"]))
        self.assertTrue(all("referenceCaptureDate" not in r for r in review["dependencies"]))

    def test_missing_non_url_reference_rejected(self):
        root = self.mutation("external-dependencies.json", lambda d: d["nonURLReferences"].pop(0))
        with self.assertRaisesRegex(ValueError, "non-URL bibliography review"):
            self.review(root)

    def test_forged_non_url_edition_rejected(self):
        root = self.mutation("external-dependencies.json", lambda d: d["nonURLReferences"][0].update(citation="ANSI X3.4, 2027"))
        with self.assertRaisesRegex(ValueError, "non-URL bibliography provenance"):
            self.review(root)

    def test_title_matched_url_cannot_upgrade_missing_source(self):
        root = self.mutation("external-dependencies.json", lambda d: d["nonURLReferences"][1].update(url="https://www.w3.org/TR/dpub-aria/"))
        with self.assertRaisesRegex(ValueError, "non-URL bibliography provenance"):
            self.review(root)

    def test_changed_historical_review_input_rejected(self):
        root = self.mutation("reviews/dependencies-original-input.json", lambda d: d["dependencies"].pop())
        with self.assertRaisesRegex(ValueError, "historical dependency review input identity"):
            self.review(root)

    def test_original_proposal_cannot_claim_download(self):
        root = self.mutation("reviews/parent-dependencies-original.json", lambda d: d["decisions"][0].update(status="archived"))
        with self.assertRaisesRegex(ValueError, "historical dependency review changed source"):
            self.review(root)

    def test_omitted_current_url_citation_rejected(self):
        root = self.mutation("external-dependencies.json", lambda d: d["dependencies"][0]["citations"].pop())
        with self.assertRaisesRegex(ValueError, "current dependency evidence"):
            self.review(root)


if __name__ == "__main__":
    unittest.main()
