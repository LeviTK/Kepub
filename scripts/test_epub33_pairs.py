"""Adversarial offline mutations of the archived paired-publication evidence."""
import json
from pathlib import Path
import tempfile
import unittest

import epub33_tests as official
from epub33_assets import ROOT, write_json


class PairedEvidence(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / "original").symlink_to(ROOT / "original", target_is_directory=True)
        dest = self.root / "official-tests"
        dest.mkdir()
        for path in (ROOT / "official-tests").iterdir():
            if path.name != "index.json":
                (dest / path.name).symlink_to(path, target_is_directory=path.is_dir())
        self.index = json.loads((ROOT / "official-tests/index.json").read_text())
        self.case = next(r for r in self.index["cases"] if r["id"] == "pkg-unique-id")

    def verify(self):
        write_json(self.root / "official-tests/index.json", self.index)
        return official.verify(self.root)

    def test_deleted_pair_cannot_pass_zero_gaps(self):
        self.case.pop("pairedFixtures")
        with self.assertRaisesRegex(ValueError, "required paired publication"):
            self.verify()

    def test_forged_pair_commit_rejected(self):
        self.case["pairedFixtures"][0]["sourceCommit"] = "0" * 40
        with self.assertRaisesRegex(ValueError, "fixed source identity drift"):
            self.verify()

    def test_forged_pair_artifact_hash_rejected(self):
        self.case["pairedFixtures"][0]["generatedSHA256"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "official test hash drift"):
            self.verify()

    def test_missing_pair_file_rejected(self):
        self.case["pairedFixtures"][0]["sourceArchivePath"] = "official-tests/missing.tar"
        with self.assertRaises(FileNotFoundError):
            self.verify()


if __name__ == "__main__":
    unittest.main()
