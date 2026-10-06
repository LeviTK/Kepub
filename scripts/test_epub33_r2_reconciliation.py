"""R2 bounded source completeness and review retention, not natural-language judgment."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest

import epub33_assets as assets
import epub33_tests as official
import epub33_upstreams as upstreams


class Reconciliation(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())

    def temporary_root(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        root = Path(tmp.name)
        for path in assets.ROOT.iterdir():
            if path.is_dir() and path.name != "reviews":
                (root / path.name).symlink_to(path, target_is_directory=True)
            elif path.is_dir():
                shutil.copytree(path, root / path.name)
            else:
                shutil.copyfile(path, root / path.name)
        return root

    def test_omitted_entire_reviewed_family_rejected_without_surviving_trigger(self):
        changed = copy.deepcopy(self.matrix)
        prefix = "/html[1]/body[1]/section[5]/section[6]/ul[1]/"
        for key in ("rows", "manualConstraints"):
            changed[key] = [r for r in changed[key] if not
                            (r["document"] == "epub" and r["domPath"].startswith(prefix))]
        with self.assertRaisesRegex(ValueError, "source member"):
            assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_postconditions_of_multi_paragraph_steps_are_independent(self):
        for number in (2, 3, 5):
            changed = copy.deepcopy(self.matrix)
            path = f"/html[1]/body[1]/section[13]/ol[1]/li[{number}]/p[2]"
            for key in ("rows", "manualConstraints"):
                changed[key] = [r for r in changed[key] if not
                                (r["document"] == "rs" and r["domPath"] == path)]
            with self.subTest(step=number), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_entire_formal_algorithm_cannot_disappear(self):
        changed = copy.deepcopy(self.matrix)
        prefix = "/html[1]/body[1]/section[13]/ol[1]/"
        for key in ("rows", "manualConstraints"):
            changed[key] = [r for r in changed[key] if not
                            (r["document"] == "rs" and r["domPath"].startswith(prefix))]
        with self.assertRaisesRegex(ValueError, "source member"):
            assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_wide_ancestor_or_duplicate_sibling_cannot_replace_member(self):
        dom = assets.DOM((assets.ROOT / "original/epub.html").read_text()).root
        nodes = {n.path(): n for n in dom.walk()}
        entry = next(e for e in json.loads((assets.ROOT / "manifest.json").read_text())["assets"]
                     if e["id"] == "epub")
        prefix = "/html[1]/body[1]/section[5]/section[6]/ul[1]"
        removed = prefix + "/li[2]"
        for substitute in (prefix, prefix + "/li[1]"):
            changed = copy.deepcopy(self.matrix)
            for key in ("rows", "manualConstraints"):
                changed[key] = [r for r in changed[key] if not
                                (r["document"] == "epub" and r["domPath"] == removed)]
            clause = assets.candidate(entry, nodes[substitute], nodes[substitute],
                                      "manual-normative", 2, "UNMARKED NORMATIVE")
            template = next(r for r in changed["rows"] if r["document"] == "epub" and
                            r["domPath"] == prefix + "/li[1]")
            changed["manualConstraints"].append(clause)
            changed["rows"].append({**{k: v for k, v in template.items() if k not in
                                      ("contextDOM", "contextExcerpt", "contextSHA256")}, **clause})
            with self.subTest(substitute=substitute), self.assertRaisesRegex(ValueError, "source member"):
                assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_explanation_cannot_replace_its_host_step(self):
        changed = copy.deepcopy(self.matrix)
        dom = assets.DOM((assets.ROOT / "original/epub.html").read_text()).root
        nodes = {n.path(): n for n in dom.walk()}
        path = "/html[1]/body[1]/section[6]/section[2]/section[5]/ol[1]/li[9]"
        template = next(r for r in changed["rows"] if r["document"] == "epub" and r["domPath"] == path)
        for key in ("rows", "manualConstraints"):
            changed[key] = [r for r in changed[key] if r["featureId"] != template["featureId"]]
        explanation = nodes[path + "/details[1]/p[1]"]
        entry = next(e for e in json.loads((assets.ROOT / "manifest.json").read_text())["assets"]
                     if e["id"] == "epub")
        clause = assets.candidate(entry, explanation, explanation, "manual-normative", 1, "EXPLANATION")
        changed["manualConstraints"].append(clause)
        changed["rows"].append({**template, **clause, "decision": "excluded", "phase": "S0-excluded",
                                "reason": "Keep the actual informative explanation, not the missing host rule"})
        with self.assertRaisesRegex(ValueError, "source member"):
            assets.verify_mapping(assets.ROOT, matrix=changed)

    def test_pending_section_can_remain_unfinished_but_gate_rejects(self):
        changed = copy.deepcopy(self.matrix)
        prefix = "/html[1]/body[1]/section[5]/section[6]/ul[1]/"
        for key in ("rows", "manualConstraints"):
            changed[key] = [r for r in changed[key] if not
                            (r["document"] == "epub" and r["domPath"].startswith(prefix))]
        section = next(r for r in changed["sectionReviews"] if r["document"] == "epub" and
                       r["anchor"] == "sec-resource-locations")
        section["status"] = "pending"
        assets.verify_mapping(assets.ROOT, matrix=changed)
        with self.assertRaisesRegex(ValueError, "pending clause/section"):
            assets.verify_mapping(assets.ROOT, gate=True, matrix=changed)

    def test_inline_whitespace_preserved_while_explanations_are_removed(self):
        root = assets.DOM('<ol><li>see <a><span>5.2 </span><span>Parsing URLs</span></a>'
                          '<details><summary>Explanation</summary><p>not normative</p></details>.</li></ol>').root
        step = next(n for n in root.walk() if n.tag == "li")
        self.assertEqual(assets.source_member_text(step), "see 5.2 Parsing URLs.")

    def test_review_import_cannot_silently_delete_existing_identity(self):
        root = self.temporary_root()
        before = (root / "matrix.json").read_bytes()
        path = root / "reviews/kepub-s0-epub-semantic-r2.json"
        review = json.loads(path.read_text())
        fid = "epub:manual-normative:/html[1]/body[1]/section[6]/section[2]/section[5]/ol[1]/li[8]:1"
        for key in ("rows", "manualConstraints"):
            review[key] = [r for r in review[key] if r["featureId"] != fid]
        assets.write_json(path, review)
        amendments = [root / "reviews" / name for name in
                      ("publication-amendments.json", "r1-publication-amendments.json",
                       "r1-accessibility-amendments.json", "r2-publication-amendments.json")
                      if (root / "reviews" / name).exists()]
        with self.assertRaisesRegex(ValueError, "existing review identities"):
            assets.import_reviews(root, [path], amendments)
        self.assertEqual((root / "matrix.json").read_bytes(), before)

    def test_top_report_commit_and_upstream_role_phase_are_bound(self):
        for field in ("reportCommit", "role", "phase", "group"):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                if field == "reportCommit":
                    for path in assets.ROOT.iterdir():
                        if path.name != "official-tests":
                            (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
                    dest = root / "official-tests"
                    dest.mkdir()
                    for path in (assets.ROOT / "official-tests").iterdir():
                        if path.name != "index.json":
                            (dest / path.name).symlink_to(path, target_is_directory=path.is_dir())
                    data = json.loads((assets.ROOT / "official-tests/index.json").read_text())
                    data[field] = "0" * 40
                    assets.write_json(dest / "index.json", data)
                    with self.assertRaisesRegex(ValueError, "report commit"):
                        official.verify(root)
                else:
                    original = assets.ROOT / "upstreams"
                    for path in original.iterdir():
                        if path.name != "index.json":
                            (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
                    data = json.loads((original / "index.json").read_text())
                    data["projects"][0][field] = "forged Workspace adoption"
                    assets.write_json(root / "index.json", data)
                    with self.assertRaisesRegex(ValueError, "upstream role/phase"):
                        upstreams.verify(root)


if __name__ == "__main__":
    unittest.main(verbosity=2)
