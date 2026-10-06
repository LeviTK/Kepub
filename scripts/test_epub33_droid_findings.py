"""Independent R1 regressions. Synthetic approval fixtures are not S0 approval."""
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import epub33_assets as assets
import epub33_semantics as semantics
import epub33_tests as official
import epub33_upstreams as upstreams


class AcceptanceProtocol(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.project = Path(self.temporary.name)
        self.root = self.project / "docs/specs/epub-3.3"
        self.root.mkdir(parents=True)
        assets.write_json(self.root / "semantic-index.json", {})
        (self.root / "original").mkdir()
        (self.root / "original/source.html").write_text("Fixed source fixture")
        (self.root / "matrix.json").write_text("{}\n")
        (self.project / "scripts").mkdir()
        (self.project / "scripts/check.py").write_text("# checker fixture\n")
        for name in ("go.mod", "go.sum", ".agents/setup", "README.md", "docs/DEVELOPMENT_PLAN.md",
                     "docs/CLI_CONTRACT.md", "docs/EPUB33_SUPPORT_MATRIX.md",
                     "docs/verification/S0_EPUBCHECK_2026.md"):
            path = self.project / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("Synthetic fixture, not a product/checker/review PASS\n")
        self.project_patch = patch.object(assets, "PROJECT", self.project)
        self.project_patch.start()
        self.addCleanup(self.project_patch.stop)
        self.derived = patch.object(assets, "verify_derived")
        self.semantic = patch.object(semantics, "build", return_value={})
        self.upstream = patch.object(upstreams, "verify")
        self.derived_mock = self.derived.start()
        self.semantic_mock = self.semantic.start()
        self.upstream_mock = self.upstream.start()
        self.addCleanup(self.derived.stop)
        self.addCleanup(self.semantic.stop)
        self.addCleanup(self.upstream.stop)
        self.make_approval()

    def make_approval(self, decision=None):
        inputs = assets.acceptance_inputs(self.root)
        self.identity = assets.sha(json.dumps(inputs, sort_keys=True, separators=(",", ":")).encode())
        decision = decision or {"scope": "complete-S0", "decision": "approved", "readingComplete": True,
                                "findings": [], "inputIdentitySHA256": self.identity}
        text = "Synthetic review only\n```json\n" + json.dumps(decision) + "\n```\n"
        (self.project / "droid.md").write_text(text)
        parent_decision = {"scope": "complete-S0", "decision": "approved", "findings": [],
                           "inputIdentitySHA256": self.identity}
        (self.project / "parent.md").write_text("Synthetic independent parent approval fixture\n```json\n" +
                                               json.dumps(parent_decision) + "\n```\n")
        (self.project / "droid.exit").write_text("0\n")
        self.events = [
            {"type": "system", "subtype": "init", "model": "claude-opus-5-5",
             "reasoning_effort": "medium", "session_id": "fixture-session"},
            {"type": "completion", "session_id": "fixture-session", "finalText": text},
        ]
        (self.project / "droid.jsonl").write_text("\n".join(json.dumps(e) for e in self.events) + "\n")
        reviews = {}
        # Droid first, parent last: iteration order must not select parent's report.
        for role in ("droid", "parent"):
            reviews[role] = {"decision": "approved", "scope": "complete-S0", "reviewer": role,
                             "inputIdentitySHA256": self.identity, "reportPath": f"{role}.md",
                             "reportSHA256": assets.sha((self.project / f"{role}.md").read_bytes())}
        reviews["droid"].update(streamPath="droid.jsonl", streamSHA256=assets.sha((self.project / "droid.jsonl").read_bytes()),
                                exitPath="droid.exit", exitSHA256=assets.sha((self.project / "droid.exit").read_bytes()),
                                cliVersion="0.233.0", sessionId="fixture-session")
        self.receipt = {"schemaVersion": 1, "inputs": inputs, "reviews": reviews}
        assets.write_json(self.root / "acceptance.json", self.receipt)

    def test_current_identity_full_decision_positive_control(self):
        self.assertEqual(assets.verify_acceptance(self.root), self.identity)
        self.assertTrue(self.derived_mock.called)
        self.assertTrue(self.semantic_mock.called)
        self.assertTrue(self.upstream_mock.called)

    def test_stale_source_matrix_index_and_checker_approvals_fail(self):
        for path in (self.root / "original/source.html", self.root / "matrix.json",
                     self.root / "semantic-index.json", self.project / "scripts/check.py"):
            with self.subTest(path=path):
                before = path.read_bytes()
                path.write_bytes(before + b" ")
                with self.assertRaisesRegex(ValueError, "input identity changed"):
                    assets.verify_acceptance(self.root)
                path.write_bytes(before)
                self.assertEqual(assets.verify_acceptance(self.root), self.identity)

    def test_referenced_execution_record_change_invalidates_approval(self):
        (self.project / "slice.json").write_text('{"result":"passed"}\n')
        assets.write_json(self.root / "matrix.json", {"rows": [{"evidence": ["slice.json"]}]})
        self.make_approval()
        self.assertEqual(assets.verify_acceptance(self.root), self.identity)
        (self.project / "slice.json").write_text('{"result":"failed"}\n')
        with self.assertRaisesRegex(ValueError, "input identity changed"):
            assets.verify_acceptance(self.root)

    def test_r1_rejection_and_partial_reading_are_not_approval(self):
        for change in ({"decision": "rejected"}, {"readingComplete": False}, {"findings": ["F1"]}):
            with self.subTest(change=change):
                decision = {"scope": "complete-S0", "decision": "approved", "readingComplete": True,
                            "findings": [], "inputIdentitySHA256": self.identity, **change}
                self.make_approval(decision)
                with self.assertRaisesRegex(ValueError, "actual Droid review not complete/approved"):
                    assets.verify_acceptance(self.root)

    def test_receipt_approval_cannot_relabel_parent_pending_report(self):
        path = self.project / "parent.md"
        path.write_text('Pending independent acceptance\n```json\n' + json.dumps({
            "scope": "complete-S0", "decision": "pending", "findings": [],
            "inputIdentitySHA256": self.identity}) + '\n```\n')
        self.receipt["reviews"]["parent"]["reportSHA256"] = assets.sha(path.read_bytes())
        assets.write_json(self.root / "acceptance.json", self.receipt)
        with self.assertRaisesRegex(ValueError, "actual parent review not complete/approved"):
            assets.verify_acceptance(self.root)

    def test_receipt_cannot_replace_actual_completion_or_exit(self):
        for kind in ("model", "session", "completion", "exit", "parent"):
            with self.subTest(kind=kind):
                self.make_approval()
                if kind == "model":
                    self.events[0]["model"] = "another-model"
                if kind == "session":
                    self.events[1]["session_id"] = "other-session"
                if kind == "completion":
                    self.events.pop()
                if kind in ("model", "session", "completion"):
                    path = self.project / "droid.jsonl"
                    path.write_text("\n".join(json.dumps(e) for e in self.events) + "\n")
                    self.receipt["reviews"]["droid"]["streamSHA256"] = assets.sha(path.read_bytes())
                if kind == "exit":
                    path = self.project / "droid.exit"
                    path.write_text("1\n")
                    self.receipt["reviews"]["droid"]["exitSHA256"] = assets.sha(path.read_bytes())
                if kind == "parent":
                    self.receipt["reviews"]["parent"]["decision"] = "pending"
                assets.write_json(self.root / "acceptance.json", self.receipt)
                with self.assertRaises(ValueError):
                    assets.verify_acceptance(self.root)

    def test_required_validator_failure_not_overruled_by_approval(self):
        self.upstream_mock.side_effect = ValueError("license provenance drift")
        with self.assertRaisesRegex(ValueError, "license provenance drift"):
            assets.verify_acceptance(self.root)

    def make_amp_approval(self, decision=None):
        self.make_approval(decision)
        text = (self.project / "droid.md").read_text()
        (self.project / "amp.md").write_text(text)
        self.export = {
            "v": 5, "id": "T-00000000-0000-0000-0000-000000000001",
            "meta": {"threadAgent": {
                "type": "custom-agent", "pluginAgentModeKey": "deepseek-v4.1-flash",
                "definition": {"name": "deepseek-v4.1-flash", "model": "deepseek/deepseek-v4.1-flash"}}},
            "messages": [
                {"role": "user", "content": [{"type": "text", "text": "Synthetic review request"}]},
                {"role": "assistant", "usage": {"model": "deepseek-v4.1-flash"},
                 "state": {"type": "complete", "stopReason": "tool_use"}, "content": []},
                {"role": "user", "content": []},
                {"role": "assistant", "usage": {"model": "deepseek-v4.1-flash"},
                 "state": {"type": "complete", "stopReason": "end_turn"},
                 "content": [{"type": "text", "text": text}]},
            ],
        }
        del self.receipt["reviews"]["droid"]
        self.receipt["reviews"]["amp"] = {
            "decision": "approved", "scope": "complete-S0", "reviewer": "Amp DeepSeek fixture",
            "inputIdentitySHA256": self.identity, "reportPath": "amp.md",
            "reportSHA256": assets.sha(text.encode()), "exportPath": "amp.json",
            "threadId": self.export["id"],
        }
        self.save_amp_export()

    def save_amp_export(self):
        path = self.project / "amp.json"
        assets.write_json(path, self.export)
        self.receipt["reviews"]["amp"]["exportSHA256"] = assets.sha(path.read_bytes())
        assets.write_json(self.root / "acceptance.json", self.receipt)

    def test_amp_current_identity_and_parent_are_still_required(self):
        self.make_amp_approval()
        self.assertEqual(assets.verify_acceptance(self.root), self.identity)
        self.assertTrue(self.derived_mock.called)
        self.assertTrue(self.semantic_mock.called)
        self.assertTrue(self.upstream_mock.called)
        # Actual exports changed v from 5 at creation to 589 during execution;
        # it is not a fixed transcript-format version.
        self.export["v"] = 589
        self.save_amp_export()
        self.assertEqual(assets.verify_acceptance(self.root), self.identity)
        path = self.project / "scripts/check.py"
        before = path.read_bytes()
        path.write_bytes(before + b"# changed\n")
        with self.assertRaisesRegex(ValueError, "input identity changed"):
            assets.verify_acceptance(self.root)
        path.write_bytes(before)
        del self.receipt["reviews"]["parent"]
        self.save_amp_export()
        with self.assertRaises(ValueError):
            assets.verify_acceptance(self.root)

    def test_amp_metadata_cannot_replace_actual_model_completion_or_report(self):
        changes = (
            lambda e: e.update(id="T-other"),
            lambda e: e["meta"]["threadAgent"].update(pluginAgentModeKey="another-mode"),
            lambda e: e["meta"]["threadAgent"]["definition"].update(model="another-model"),
            lambda e: e["messages"][1]["usage"].update(model="another-model"),
            lambda e: e["messages"][-1].pop("usage"),
            lambda e: e["messages"][-1].update(state={"type": "streaming"}),
            lambda e: e["messages"][-1]["state"].update(stopReason="tool_use"),
            lambda e: e["messages"][-1]["content"][0].update(text="Only a delivery acknowledgment"),
            lambda e: e["messages"].append({"role": "user", "content": []}),
            lambda e: e.update(messages=[]),
        )
        for i, change in enumerate(changes):
            with self.subTest(mutation=i):
                self.make_amp_approval()
                change(self.export)
                # Rehash the changed export: rejection must inspect its contents.
                self.save_amp_export()
                with self.assertRaisesRegex(ValueError, "Amp"):
                    assets.verify_acceptance(self.root)

    def test_amp_rejected_partial_or_wrong_scope_report_is_not_approval(self):
        for change in ({"decision": "rejected"}, {"readingComplete": False},
                       {"findings": ["F1"]}, {"scope": "gate-only"}):
            with self.subTest(change=change):
                decision = {"scope": "complete-S0", "decision": "approved", "readingComplete": True,
                            "findings": [], "inputIdentitySHA256": self.identity, **change}
                self.make_amp_approval(decision)
                with self.assertRaisesRegex(ValueError, "actual Amp review not complete/approved"):
                    assets.verify_acceptance(self.root)


class EvidenceReferences(unittest.TestCase):
    def test_completion_boolean_cannot_bypass_real_aggregate_gate(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "epub-3.3"
            shutil.copytree(assets.ROOT, root)
            matrix = json.loads((root / "matrix.json").read_text())
            matrix["semanticComplete"] = True
            assets.write_json(root / "matrix.json", matrix)
            assets.inventory(root)
            assets.write_json(root / "semantic-index.json", semantics.build(root))
            # All underlying validators have valid input. Failure must be missing
            # actual independent approval, not a stale derivative or broken EPUB.
            assets.verify_mapping(root)
            assets.verify_derived(root)
            self.assertEqual(official.verify(root)["sourceGaps"], 0)
            with self.assertRaisesRegex(ValueError, "independent acceptance records missing"):
                assets.verify_mapping(root, gate=True)

    def test_real_test_reference_exists_but_is_not_execution(self):
        reference = "internal/archive/snapshot_test.go:TestPackHeadersBytesAndNoClobber"
        self.assertTrue(assets.repository_evidence(reference).is_file())
        matrix = json.loads((assets.ROOT / "matrix.json").read_text())
        row = next(r for r in matrix["rows"] if r["decision"] == "mapped")
        row.update(evidence=[reference], testIds=[reference])
        assets.verify_mapping(assets.ROOT, matrix=matrix)
        row["parse"] = "supported"
        with self.assertRaisesRegex(ValueError, "clause-specific execution evidence"):
            assets.verify_mapping(assets.ROOT, matrix=matrix)

    def test_missing_file_test_and_outside_references_fail(self):
        for reference in ("internal/does-not-exist_test.go:TestImaginary", "../README.md", "/etc/passwd",
                          "internal/archive/snapshot_test.go:TestImaginary"):
            with self.subTest(reference=reference), self.assertRaises(ValueError):
                assets.repository_evidence(reference)


class SourceOnlyClaims(unittest.TestCase):
    def mutate_index(self, relative, mutate):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        for path in assets.ROOT.iterdir():
            if path.name == relative.split("/")[0]:
                shutil.copytree(path, root / path.name)
            else:
                (root / path.name).symlink_to(path, target_is_directory=path.is_dir())
        path = root / relative
        index = json.loads(path.read_text())
        mutate(index)
        assets.write_json(path, index)
        return root

    def test_official_actual_false_positive_control(self):
        self.assertFalse(official.verify(assets.ROOT)["executed"])
        self.assertEqual(upstreams.verify(assets.ROOT / "upstreams")["gaps"], 1)

    def test_official_execution_result_and_paired_forgery_rejected(self):
        mutations = (
            lambda r: r["cases"][0].update(executed=True),
            lambda r: r["cases"][0].update(result="pass"),
            lambda r: r["cases"][0].update(status="executed; PASS"),
            lambda r: next(c for c in r["cases"] if c["id"] == "pkg-unique-id")["pairedFixtures"][0].update(executed=True),
        )
        for mutate in mutations:
            root = self.mutate_index("official-tests/index.json", mutate)
            with self.subTest(mutate=mutate), self.assertRaisesRegex(ValueError, "execution"):
                official.verify(root)

    def test_upstream_behavior_adoption_and_deleted_gap_rejected(self):
        mutations = (
            lambda r: r["projects"][0].update(behaviorTested=True),
            lambda r: r["projects"][0].update(adoption="adopted"),
            lambda r: r.update(failures=[]),
        )
        for mutate in mutations:
            root = self.mutate_index("upstreams/index.json", mutate)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                upstreams.verify(root / "upstreams")


if __name__ == "__main__":
    unittest.main(verbosity=2)
