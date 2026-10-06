"""Pinned NVDL/Schematron source closure, not schema execution or support."""
import copy
import json
from pathlib import Path
import tempfile
import unittest

import epub33_assets as assets


class NVDLClosure(unittest.TestCase):
    def test_tree_master_and_blob_main_links_pin_actual_file(self):
        for location in ("tree/master", "blob/main"):
            source = assets.DOM(f'<a href="https://github.com/w3c/epubcheck/{location}/schema/package.nvdl">schema</a>').root
            self.assertEqual(assets.resources(source, "https://example.test/spec/"), [
                f"https://raw.githubusercontent.com/w3c/epubcheck/{assets.EPUBCHECK_SCHEMA_COMMIT}/schema/package.nvdl"])

    def test_nvdl_and_schematron_inherited_xml_base(self):
        nvdl = b'<rules xmlns="http://purl.oclc.org/dsdl/nvdl/ns/structure/1.0" xml:base="../schema/"><mode xml:base="30/"><validate schema="one.rnc"/><validate xml:base="../../shared/" schema="two.sch"/></mode></rules>'
        self.assertEqual(assets.schema_dependencies(nvdl, "https://example.test/spec/root.nvdl"),
                         ["https://example.test/schema/30/one.rnc", "https://example.test/shared/two.sch"])
        sch = b'<schema xmlns="http://purl.oclc.org/dsdl/schematron" xml:base="mod/"><include href="id.sch"/><include xml:base="../" href="other.sch"/></schema>'
        self.assertEqual(assets.schema_dependencies(sch, "https://example.test/root.sch"),
                         ["https://example.test/mod/id.sch", "https://example.test/other.sch"])

    def test_fixed_three_dispatchers_have_exact_fifteen_file_closure(self):
        manifest = assets.verify(assets.ROOT)
        prefix = f"https://raw.githubusercontent.com/w3c/epubcheck/{assets.EPUBCHECK_SCHEMA_COMMIT}/src/main/resources/com/adobe/epubcheck/schema/30/"
        entries = {e["requestUrl"]: e for e in manifest["assets"]}
        roots = {prefix + n + ".nvdl" for n in ("package-30", "ocf-container-30", "media-overlay-30")}
        pending, found = list(roots), set()
        while pending:
            url = pending.pop()
            if url in found:
                continue
            found.add(url)
            pending.extend(entries[url]["dependencies"])
        expected = {"LICENSE", "package-30.nvdl", "package-30.rnc", "package-30.sch",
                    "ocf-container-30.nvdl", "ocf-container-30.rnc", "multiple-renditions/container.rnc",
                    "multiple-renditions/container.sch", "media-overlay-30.nvdl", "media-overlay-30.rnc",
                    "media-overlay-30.sch", "mod/id-unique.sch", "mod/datatypes.rnc",
                    "mod/epub-type-attr.rnc", "mod/epub-prefix-attr.rnc"}
        self.assertEqual(found, {prefix + p for p in expected})
        self.assertEqual(entries[prefix + "LICENSE"]["sha256"],
                         "bc31197ad4f01c1813dd200349c75309c29a1486b5b5d1ae51a5203acfc4df00")
        # Remove an actual dispatch dependency while preserving every source
        # byte and its hash. The lock cannot silently omit this direct schema.
        changed = copy.deepcopy(manifest)
        entry = next(e for e in changed["assets"] if e["requestUrl"] == prefix + "package-30.nvdl")
        entry["dependencies"].remove(prefix + "package-30.sch")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for directory in ("assets", "original"):
                (root / directory).symlink_to(assets.ROOT / directory, target_is_directory=True)
            assets.write_json(root / "manifest.json", changed)
            with self.assertRaisesRegex(ValueError, "omitted asset dependency"):
                assets.verify(root)


if __name__ == "__main__":
    unittest.main(verbosity=2)
