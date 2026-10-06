"""Manual clauses must bind version and section to the archived source itself."""
import copy
import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path.cwd() / "scripts"))
import epub33_assets as assets


class ManualProvenanceAcceptance(unittest.TestCase):
    def test_self_consistent_manual_records_cannot_relabel_source(self):
        matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        changes = (
            ("specVersion", "https://www.w3.org/TR/2027/REC-epub-34-20270101/"),
            ("specSection", "sec-nav-toc"),
        )
        for field, value in changes:
            with self.subTest(field=field):
                changed = copy.deepcopy(matrix)
                clause = next(r for r in changed["manualConstraints"]
                              if r["document"] == "epub" and r["specSection"] == "sec-item-elem")
                clause[field] = value
                row = next(r for r in changed["rows"] if r["featureId"] == clause["featureId"])
                row[field] = value
                with self.assertRaises(ValueError):
                    assets.verify_mapping(assets.ROOT, matrix=changed)


if __name__ == "__main__":
    unittest.main(verbosity=2)
