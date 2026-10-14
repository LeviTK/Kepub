"""Exercise CI routing/checking through its CLI against real Git histories."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("ci_docs.py").resolve()


class DocsCI(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.git("init", "-q")
        self.git("config", "user.email", "ci@example.invalid")
        self.git("config", "user.name", "CI fixture")
        self.write("README.md", "# Intro\n")
        self.write("docs/target.md", '# Title\n## Same\n## Same\n<a id="explicit"></a>\n')
        self.write("cmd/main.go", "package main\n")
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.root, text=True,
                                       stderr=subprocess.PIPE)

    def write(self, name, value):
        p = self.root / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(value)

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-qm", "fixture")

    def run_cli(self, command="classify", event="pull_request", base=None):
        env = dict(os.environ)
        output = self.root / ".git/action-output"
        output.unlink(missing_ok=True)
        env["GITHUB_OUTPUT"] = str(output)
        return subprocess.run([sys.executable, "-B", str(SCRIPT), command,
                               "--event", event, "--base", self.base if base is None else base],
                              cwd=self.root, env=env, text=True, capture_output=True)

    def mode(self, **kwargs):
        result = self.run_cli(**kwargs)
        self.assertEqual(result.returncode, 0, result.stderr)
        mode = json.loads(result.stdout)["mode"]
        self.assertEqual((self.root / ".git/action-output").read_text(), f"mode={mode}\n")
        return mode

    def test_explanation_allowlist_and_full_boundaries(self):
        for name, expected in [
            ("README.md", "docs"), ("docs/research/new note.md", "docs"),
            ("docs/research/AGENTS.md", "full"), ("AGENTS.md", "full"),
            ("docs/CLI_CONTRACT.md", "full"), ("docs/contracts/new.md", "full"),
            ("docs/verification/result.md", "full"), ("docs/DEVELOPMENT_WORKFLOW.md", "full"),
            ("docs/DEVELOPMENT_PLAN.md", "full"), ("docs/specs/raw.md", "full"),
            ("docs/examples/book.json", "full"), ("scripts/ci_docs.py", "full"),
            (".github/workflows/ci.yml", "full"), (".github/rulesets/main.json", "full"),
            ("go.mod", "full"), ("cmd/main.go", "full"), ("new_test.go", "full"),
        ]:
            with self.subTest(name=name):
                self.git("reset", "--hard", self.base)
                self.write(name, "new content\n")
                self.commit()
                self.assertEqual(self.mode(), expected)

    def test_mixed_deleted_renamed_and_symlink_are_full(self):
        self.write("README.md", "changed\n")
        self.write("cmd/main.go", "package changed\n")
        self.commit()
        self.assertEqual(self.mode(), "full")
        self.git("reset", "--hard", self.base)
        (self.root / "README.md").unlink()
        self.commit()
        self.assertEqual(self.mode(), "full")
        self.git("reset", "--hard", self.base)
        (self.root / "docs/research").mkdir(parents=True, exist_ok=True)
        self.git("mv", "README.md", "docs/research/moved.md")
        self.commit()
        self.assertEqual(self.mode(), "full")
        self.git("reset", "--hard", self.base)
        (self.root / "README.md").unlink()
        (self.root / "README.md").symlink_to("docs/target.md")
        self.commit()
        self.assertEqual(self.mode(), "full")

    def test_executable_mode_and_invalid_utf8_fail_closed(self):
        (self.root / "README.md").chmod(0o755)
        self.commit()
        self.assertEqual(self.mode(), "full")
        self.git("reset", "--hard", self.base)
        (self.root / "README.md").write_bytes(b"invalid \xff\n")
        self.commit()
        self.assertEqual(self.mode(), "docs")
        result = self.run_cli("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("UnicodeDecodeError", result.stderr)

    def test_missing_base_manual_empty_and_nonancestor_are_full(self):
        self.assertEqual(self.mode(), "full")
        self.write("README.md", "changed\n")
        self.commit()
        self.assertEqual(self.mode(base="missing-ref"), "full")
        self.assertEqual(self.mode(event="workflow_dispatch"), "full")
        newer = self.git("rev-parse", "HEAD").strip()
        self.git("reset", "--hard", self.base)
        self.assertEqual(self.mode(base=newer), "full")

    def test_real_link_checks_and_product_not_run(self):
        self.write("README.md", '[inline](docs/target.md#title)\n[ref][target]\n'
                   '[target]: docs/target.md#same-1\n<a href="docs/target.md#explicit">x</a>\n'
                   '```md\n[example](not-a-real-file)\n```\n')
        self.commit()
        result = self.run_cli("check")
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        self.assertEqual(output["local_links_checked"], 3)
        self.assertEqual(output["product_tests"], "not_run")
        for link in ["docs/missing.md", "docs/target.md#missing", "../outside.md"]:
            with self.subTest(link=link):
                self.write("README.md", f"[bad]({link})\n")
                self.commit()
                result = self.run_cli("check")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("link" if "#" not in link else "anchor", result.stderr)

    def test_removed_heading_breaks_incoming_link(self):
        self.write("docs/research/note.md", "# Original heading\n")
        self.write("docs/target.md", "[incoming](research/note.md#original-heading)\n")
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()
        self.write("docs/research/note.md", "# Replacement heading\n")
        self.commit()
        self.assertEqual(self.mode(), "docs")
        result = self.run_cli("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing Markdown anchor", result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
