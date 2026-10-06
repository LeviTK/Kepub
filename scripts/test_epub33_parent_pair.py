"""Independent paired-book inventory check; execute from the repository root."""
import hashlib
import json
from pathlib import Path
import tarfile
import unittest
import zipfile


ROOT = Path("docs/specs/epub-3.3")
# Independently downloaded source files at report commit 54092b4233253e9aac80e93ec4782b380b4b3403.
EXPECTED = {
    "mimetype": "e468e350d1143eb648f60c7b0bd6031101ec0544a361ca74ecef256ac901f48b",
    "META-INF/container.xml": "b871e7745e5d2bfa067a032b9b9a771185adcf0e8173b36fdbeed181163ef93c",
    "EPUB/content_001.xhtml": "0c29d7b30867277bd90831653c837fe3e634f623abdf22d11e87031c8b83362b",
    "EPUB/package.opf": "1f6798518ba915b268f02db3e494c577e853ac6a0e2b89da1f8d59a713db2aeb",
    "EPUB/nav.xhtml": "97787442936a97c67b68d7b53652c493ba734e36e39881d067a3e9f554e01b5f",
}


def objects(value):
    if isinstance(value, dict):
        yield value
        for item in value.values():
            yield from objects(item)
    elif isinstance(value, list):
        for item in value:
            yield from objects(item)


class PairedFixtureAcceptance(unittest.TestCase):
    def test_required_second_publication_has_source_and_both_artifacts(self):
        index = json.loads((ROOT / "official-tests/index.json").read_text())
        rows = [r for r in objects(index) if r.get("sourcePath") == "tests/pkg-unique-id_duplicate"]
        self.assertTrue(rows, "Duplicate report ID must not discard the required second publication")
        row = rows[0]
        self.assertTrue(row["sourceCommit"])
        with tarfile.open(ROOT / row["sourceArchivePath"]) as archive:
            actual = {m.name: hashlib.sha256(archive.extractfile(m).read()).hexdigest()
                      for m in archive if m.isfile()}
        self.assertEqual(actual, EXPECTED)
        official = row.get("websiteArtifactPath") or row.get("historicalArtifactPath")
        self.assertTrue(official, "Source alone is not an archived official binary")
        for path in (official, row["generatedArtifactPath"]):
            with self.subTest(path=path), zipfile.ZipFile(ROOT / path) as archive:
                actual = {i.filename: hashlib.sha256(archive.read(i)).hexdigest()
                          for i in archive.infolist() if not i.is_dir()}
            self.assertEqual(actual, EXPECTED)


if __name__ == "__main__":
    unittest.main(verbosity=2)
