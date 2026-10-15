"""Exercise CI routing/checking through its CLI against real Git histories."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest


SCRIPT = Path(__file__).with_name("ci_docs.py").resolve()
WORKFLOW = Path(__file__).resolve().parents[1] / ".github/workflows/ci.yml"


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
        original_base = self.base
        self.base = self.git("rev-parse", "HEAD").strip()
        (self.root / "README.md").chmod(0o644)
        self.commit()
        self.assertEqual(self.mode(), "full")
        self.base = original_base
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
                   '`<item href="not-a-target"/>` and `[example](missing)`\n'
                   '``example with ` and [example](also-missing)``\n'
                   '<!-- [hidden](not-published) -->\n'
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

    def test_code_span_cannot_hide_link_in_next_paragraph(self):
        self.write("README.md", 'stray ` tick\n\n[broken](docs/nonexistent-zz.md)\n\nlater `code`\n')
        self.commit()
        result = self.run_cli("check")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("docs/nonexistent-zz.md", result.stderr)
        self.write("README.md", '`multiline example\n[example](missing)`\n\n'
                   '[real](docs/target.md#title)\n')
        self.commit()
        result = self.run_cli("check")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["local_links_checked"], 1)

    def test_workflow_uses_base_classifier_not_modified_head(self):
        # Execute the actual workflow's classification shell, not a second
        # Python implementation of its trust boundary.
        step = re.search(r"^      - name: Classify exact PR changes \(unknown means full\)\n"
                         r".*?^        run: \|\n((?:^          [^\n]*\n|^\n)+)",
                         WORKFLOW.read_text(), re.M | re.S)
        self.assertIsNotNone(step)
        shell = textwrap.dedent(step[1])
        env = dict(os.environ, CI_EVENT="pull_request", CI_BASE=self.base,
                   RUNNER_TEMP=str(self.root / ".git/runner"),
                   GITHUB_OUTPUT=str(self.root / ".git/scope-output"),
                   GITHUB_STEP_SUMMARY=str(self.root / ".git/summary"))

        def workflow_mode():
            Path(env["GITHUB_OUTPUT"]).unlink(missing_ok=True)
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", shell],
                                    cwd=self.root, env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            mode = json.loads((Path(env["RUNNER_TEMP"]) / "results/scope.json").read_text())["mode"]
            self.assertEqual(Path(env["GITHUB_OUTPUT"]).read_text(), f"mode={mode}\n")
            return mode

        initial_base = self.base
        source = SCRIPT.read_text()
        self.write("scripts/ci_docs.py", source)
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()
        env["CI_BASE"] = self.base
        self.write("README.md", "changed\n")
        self.commit()
        self.assertEqual(workflow_mode(), "docs")
        self.write("scripts/ci_docs.py", source.replace("def route(event, base):\n",
                   'def route(event, base):\n    return "docs", []\n'))
        self.write("cmd/main.go", "package changed\n")
        self.commit()
        result = subprocess.run([sys.executable, "-B", "scripts/ci_docs.py", "classify",
                                 "--base", self.base], cwd=self.root, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["mode"], "docs")
        self.assertEqual(workflow_mode(), "full")
        # Bootstrap, missing history and manual events must not downgrade.
        for event, base in [("pull_request", initial_base), ("pull_request", ""),
                            ("pull_request", "missing-ref"), ("workflow_dispatch", self.base)]:
            env["CI_EVENT"], env["CI_BASE"] = event, base
            self.assertEqual(workflow_mode(), "full")
        # An available base classifier that crashes fails the shell step.
        self.write("scripts/ci_docs.py", 'raise RuntimeError("classifier failed")\n')
        self.commit()
        env["CI_EVENT"], env["CI_BASE"] = "pull_request", self.git("rev-parse", "HEAD").strip()
        result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", shell],
                                cwd=self.root, env=env, text=True, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("classifier failed", result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
