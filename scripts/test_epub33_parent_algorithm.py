"""The fixed REC's illustrative pseudo-code must not become normative steps."""
import copy
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


class AlgorithmExplanationAcceptance(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        cls.source = (assets.ROOT / "original/epub.html").read_bytes()
        cls.base = "/html[1]/body[1]/section[6]/section[4]/section[4]"

    def test_illustrative_steps_retained_but_excluded_and_musts_preserved(self):
        self.assertEqual(assets.sha(self.source),
                         "f927cf3182598c552037445837b73faee346f97d4a396059440830e49e737d99")
        self.assertIn(b"All algorithm explanations are <em>non-normative</em>", self.source)
        self.assertIn(b"The following pseudo-code exemplifies the obfuscation algorithm.", self.source)
        rows = {r["featureId"]: r for r in self.matrix["rows"]}
        # These nine source identities existed before the correction. Preserve
        # them as exclusions rather than hiding the original mistaken mapping.
        old_paths = [f"/ol[1]/li[{i}]" for i in range(1, 6)] + [
            "/ol[1]/li[6]/ol[1]/li[1]",
            "/ol[1]/li[6]/ol[1]/li[2]/ol[1]/li[2]",
            "/ol[1]/li[6]/ol[1]/li[2]/ol[1]/li[3]",
            "/ol[1]/li[6]/ol[1]/li[2]/ol[1]/li[4]",
        ]
        for suffix in old_paths:
            self.assertIn(f"epub:manual-normative:{self.base}{suffix}:1", rows)
        for row in rows.values():
            if row["document"] == "epub" and row["domPath"].startswith(self.base + "/ol[1]/"):
                with self.subTest(path=row["domPath"]):
                    self.assertEqual(row["decision"], "excluded")
                    self.assertTrue(row["reason"])
        for excerpt in (
            "EPUB creators MUST derive the key used in the obfuscation algorithm from the unique identifier.",
            "EPUB creators MUST obfuscate fonts before compressing and adding them to the OCF ZIP container.",
        ):
            controls = [r for r in rows.values() if r["document"] == "epub"
                        and r["kind"] == "bcp14" and r["excerpt"].startswith(excerpt)]
            self.assertTrue(controls, excerpt)
            self.assertTrue(all(r["decision"] == "mapped" for r in controls), excerpt)

    def test_verifier_rejects_repromoting_illustrative_step(self):
        assets.verify_mapping(assets.ROOT, matrix=self.matrix)
        changed = copy.deepcopy(self.matrix)
        row = next(r for r in changed["rows"] if r["featureId"] ==
                   f"epub:manual-normative:{self.base}/ol[1]/li[1]:1")
        row.update(decision="mapped", phase="T3/T5", reason="Pretend the illustrative step is mandatory",
                   applicability="Font processing", gap="No implementation behavior claim")
        with self.assertRaises(ValueError):
            assets.verify_mapping(assets.ROOT, matrix=changed)


if __name__ == "__main__":
    unittest.main(verbosity=2)
