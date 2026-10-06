#!/usr/bin/env python3
"""Rebuild/verify source-addressed S0 case and supporting-document reviews offline.

This never executes an EPUB, marks feature support, or replaces report history.
Editorial review packets are inputs; archive/matrix identities are independently
recomputed. A generated index is not independent acceptance or Factory review.
"""
import argparse
import copy
import json
from urllib.parse import urldefrag, urlsplit

import epub33_assets as assets
import epub33_tests as official


def clause_context(node):
    """Normalize source labels/cells without broadening to a whole section."""
    if node.tag == "dt":
        following = node.parent.children[node.parent.children.index(node) + 1:]
        return next((n for n in following if isinstance(n, assets.Node) and n.tag == "dd"), node)
    if node.tag not in ("p", "li", "dd", "tr", "section"):
        return next((n for n in node.ancestors() if n.tag in ("p", "li", "dd", "tr", "section")), node)
    return node


def dependency_review(root, entries, inventory, nodes):
    """Explicitly rebind historical review after P5; never rewrite its evidence."""
    path = root / "external-dependencies.json"
    current = json.loads(path.read_text())
    original_path = root / "reviews/dependencies-original-input.json"
    proposal_path = root / "reviews/parent-dependencies-original.json"
    original = json.loads(original_path.read_text())
    proposal = json.loads(proposal_path.read_text())
    if proposal["sourceInputSHA256"] != assets.sha(original_path.read_bytes()):
        raise ValueError("historical dependency review input identity drift")
    citations = {(r["document"], r["entry"]): r for r in inventory["directReferences"]}
    expected_urls = {}
    for ref in citations.values():
        for url in ref["urls"]:
            expected_urls.setdefault(urldefrag(url)[0], []).append(
                {k: ref[k] for k in ("document", "entry", "kind")})
    old = {r["url"]: r for r in original["dependencies"]}
    actual = {r["url"]: r for r in current["dependencies"]}
    decisions = {r["url"]: r for r in proposal["decisions"]}
    if (set(actual) != set(expected_urls) or set(old) != set(actual) or set(decisions) != set(actual) or
            len(actual) != len(current["dependencies"]) or len(decisions) != len(proposal["decisions"])):
        raise ValueError("missing/duplicate dependency review")
    output, revisions = [], []
    for url, record in actual.items():
        source = old[url]
        decision = copy.deepcopy(decisions[url])
        for key in ("url", "citations", "status", "sha256", "versionOrAcquiredAt"):
            if decision[key] != source[key]:
                raise ValueError("historical dependency review changed source evidence")
        archived = next((e for e in entries.values() if e["requestUrl"] == url), None)
        if (record["citations"] != expected_urls[url] or
                record["status"] != ("archived" if archived else "not-downloaded") or
                record["sha256"] != (archived["sha256"] if archived else None) or
                record["versionOrAcquiredAt"] != (archived["version"] if archived else None)):
            raise ValueError("current dependency evidence differs from fixed sources")
        previous = {(r["document"], r["entry"]): r for r in source["citations"]}
        if set(previous) != {(r["document"], r["entry"]) for r in record["citations"]}:
            raise ValueError("P5 must not remove or add historical URL citations")
        if (len(decision["citedEditions"]) != len(previous) or
                {(r["document"], r["entry"]) for r in decision["citedEditions"]} != set(previous)):
            raise ValueError("missing/duplicate dependency citation edition review")
        if "nearbyArchivedEdition" in decision:
            nearby = decision["nearbyArchivedEdition"]
            entry = entries[nearby["manifestId"]]
            if any(nearby[k] != entry[k] for k in ("finalUrl", "category", "downloadedAt", "bytes", "sha256")):
                raise ValueError("nearby archive is not the recorded edition")
        provenance = []
        for citation in record["citations"]:
            identity = (citation["document"], citation["entry"])
            ref = citations[identity]
            before = previous[identity]
            if before != citation:
                if not (identity[0] == "css" and before["kind"] == "normative" and citation["kind"] == "informative"):
                    raise ValueError("unreviewed dependency citation revision")
                revisions.append({"url": url, "before": before, "after": citation,
                                  "sourceHash": ref["sourceHash"], "sourceDOM": ref["domPath"],
                                  "reason": "P5: explicit Non-Normative References heading in the frozen CSS source"})
            edition = next((r for r in decision["citedEditions"] if
                            (r["document"], r["entry"]) == identity), None)
            if not edition or edition["bibliographyText"] != ref["citation"]:
                raise ValueError("dependency citation text differs from fixed source")
            if any(not any(n.attrs.get("id") == section for n in nodes[identity[0]].values())
                   for section in edition["citingSections"]):
                raise ValueError("dependency citing section missing from fixed source")
            provenance.append({**citation, "sourceHash": ref["sourceHash"], "sourceDOM": ref["domPath"],
                               "citation": ref["citation"],
                               "citationSourceCapturedAt": entries[identity[0]]["downloadedAt"]})
        if decision["decision"] not in ("mapped", "excluded") or not decision["reason"] or not decision["applicability"]:
            raise ValueError("dependency decision lacks applicability/reason")
        if decision["decision"] == "mapped" and not decision["phases"]:
            raise ValueError("mapped dependency lacks stage")
        if decision["decision"] == "excluded" and any(r["kind"] == "normative" for r in record["citations"]):
            raise ValueError("normative dependency exclusion requires explicit review")
        decision["citations"] = record["citations"]
        decision["citationProvenance"] = provenance
        # This date is for citing pages, never a target document acquisition.
        if "referenceCaptureDate" in decision:
            decision["citationSourceCapturedAt"] = decision.pop("referenceCaptureDate")
            if any(entries[d]["downloadedAt"] != date for d, date in
                   decision["citationSourceCapturedAt"]["byCitingDocument"].items()):
                raise ValueError("citation source date differs from archive manifest")
        decision["historicalCitationKindCaptureFlags"] = decision.pop("citationKindCaptureFlags", [])
        decision["capabilities"] = {k: "not-tested" for k in ("preserve", "parse", "edit", "render", "validate")}
        output.append(decision)

    packet = json.loads((root / "reviews/non-url-references.json").read_text())
    no_url = {(r["document"], r["entry"]): r for r in current["nonURLReferences"]}
    expected_no_url = {k for k, r in citations.items() if not r["urls"]}
    non_url_decisions = {(r["document"], r["entry"]): r for r in packet["decisions"]}
    if (set(no_url) != expected_no_url or set(non_url_decisions) != expected_no_url or
            len(no_url) != len(current["nonURLReferences"]) or len(non_url_decisions) != len(packet["decisions"])):
        raise ValueError("missing/duplicate non-URL bibliography review")
    no_url_output = []
    for identity, record in no_url.items():
        ref, decision = citations[identity], non_url_decisions[identity]
        if (any(record[k] != ref[k] for k in ("document", "entry", "kind", "citation", "sourceHash", "domPath")) or
                record["url"] is not None or record["sha256"] is not None or
                record["versionOrAcquiredAt"] is not None or record["status"] != "not-downloaded"):
            raise ValueError("non-URL bibliography provenance/target identity drift")
        if not decision["reason"] or not decision["applicability"] or not decision["phases"]:
            raise ValueError("non-URL review lacks applicability/stage/reason")
        no_url_output.append({**record, **decision,
                              "citationSourceCapturedAt": entries[identity[0]]["downloadedAt"],
                              "capabilities": {k: "not-tested" for k in ("preserve", "parse", "edit", "render", "validate")}})
    return {"schemaVersion": 1, "reviewStatus": "source-mapped; independent acceptance and Droid pending",
            "sourceInputSHA256": assets.sha(path.read_bytes()),
            "historicalReviewSHA256": assets.sha(proposal_path.read_bytes()),
            "historicalSourceInputSHA256": assets.sha(original_path.read_bytes()),
            "revisionPolicy": "Explicit P5 citation-kind correction and four separate non-URL records; original proposal unchanged",
            "citationKindRevisions": revisions, "dependencies": output, "nonURLReferences": no_url_output,
            "counts": {"urlRecords": len(output), "nonURLRecords": len(no_url_output),
                       "urlCitations": sum(len(r["citations"]) for r in output),
                       "sourceBibliographyEntries": len(citations)},
            "limits": "No new target downloads, edition equivalence, legal advice, behavior execution or feature support claimed"}


def build(root):
    manifest = assets.verify(root)
    assets.verify_mapping(root)
    assets.verify_derived(root)
    official_counts = official.verify(root)
    entries = {e["id"]: e for e in manifest["assets"]}
    inventory = json.loads((root / "inventory.json").read_text())
    matrix = json.loads((root / "matrix.json").read_text())
    legacy = json.loads((root / "reviews/t3-prerequisites.json").read_text())
    required_legacy = {"opf-201", "ops-201", "ocf-201", "ncx", "dtbook", "xhtml11-dtd", "xhtml11-modules", "xhtml11-entities"}
    if ({r["id"] for r in legacy["requirements"]} != required_legacy or len(legacy["requirements"]) != 8 or
            legacy["schemaVersion"] != 1 or legacy["phase"] != "T3" or
            legacy["registrationOnly"] is not True or legacy["runtimeDownloads"] is not False or not legacy["requiredBefore"]):
        raise ValueError("missing/duplicate or misrepresented finite T3 prerequisites")
    citations = {(r["document"], r["entry"]): r for r in inventory["directReferences"]}
    legacy_citations = {"opf-201": ("epub", "bib-opf-201"), "ops-201": ("overview", "bib-ops-201"),
                        "ocf-201": ("overview", "bib-ocf-201")}
    for record in legacy["requirements"]:
        if (record["status"] != "pending-T3-archive" or record["sha256"] is not None or
                record["archivePath"] is not None or any(not record[k] for k in ("title", "edition", "gap"))):
            raise ValueError("T3 registration must not invent archived evidence")
        if record["id"] in legacy_citations:
            identity = legacy_citations[record["id"]]
            reference = citations[identity]
            if (record["citationSource"] != dict(zip(("document", "entry"), identity)) or
                    record["url"] not in reference["urls"]):
                raise ValueError("T3 legacy citation differs from fixed source")
            record["citationProvenance"] = {k: reference[k] for k in ("document", "entry", "sourceHash", "domPath", "citation")}
        elif record["url"] is not None:
            raise ValueError("T3 unresolved family target must not invent an edition URL")
    nodes = {}
    for document, entry in entries.items():
        if not entry["path"].startswith("original/") or "html" not in entry["contentType"]:
            continue
        dom = assets.DOM((root / entries[document]["path"]).read_text()).root
        nodes[document] = {n.path(): n for n in dom.walk()}

    supporting = json.loads((root / "reviews/supporting.json").read_text())
    expected_documents = {"css", "overview", "techniques", "ssv", "aria", "multi-rend", "tts", "eaa"}
    if {r["id"] for r in supporting["sourceIdentities"]} != expected_documents or len(supporting["sourceIdentities"]) != 8:
        raise ValueError("missing/duplicate supporting document review")
    for identity in supporting["sourceIdentities"]:
        document = identity["id"]
        if (identity["htmlSHA256"] != entries[document]["sha256"] or
                identity["textSHA256"] != assets.sha((root / f"text/{document}.txt").read_bytes())):
            raise ValueError("supporting source/derivative identity drift")
    expected = {(r["document"], r["domPath"]) for r in inventory["sections"] if r["document"] in expected_documents}
    actual = {(r["document"], r["sourceDOM"]) for r in supporting["sectionReviews"]}
    if not expected <= actual or len(actual) != len(supporting["sectionReviews"]):
        raise ValueError("missing/duplicate supporting section review")
    for record in supporting["sectionReviews"]:
        document = record["document"]
        node = nodes[document].get(record["sourceDOM"])
        excerpt = record["excerpt"]
        context = [node] if node else []
        if node and node.tag in ("h1", "h2", "h3", "h4", "h5", "h6"):
            # Bikeshed's flat sections are heading plus following siblings,
            # unlike ReSpec's nested <section> nodes. Retain an explicit range.
            siblings = node.parent.children[node.parent.children.index(node) + 1:]
            for sibling in siblings:
                if not isinstance(sibling, assets.Node):
                    continue
                if sibling.tag in ("h1", "h2", "h3", "h4", "h5", "h6") and sibling.tag <= node.tag:
                    break
                context.append(sibling)
        source_text = " ".join(assets.normalized(n) for n in context)
        # Display excerpts explicitly terminate with an ellipsis when truncated.
        prefix = excerpt[:-1].rstrip() if excerpt.endswith("…") else excerpt
        if (not node or not prefix or prefix not in source_text or
                record["sourceHash"] != entries[document]["sha256"] or
                assets.sha(excerpt.encode()) != record["excerptUTF8SHA256"]):
            raise ValueError(f"supporting section excerpt drift: {document}/{record['anchor']}")
        record["sourceRangeDOM"] = [n.path() for n in context]
        if record["featureOrSupportDeclaration"] or any(v != "not-tested" for v in record["capabilities"].values()):
            raise ValueError("source review must not invent feature evidence")
        if any(not record.get(k) for k in ("plannedPhase", "applicability", "conditionsAndBoundary", "reviewExplanation")):
            raise ValueError("supporting section lacks applicability/stage/review")
    expected_modules = set()
    for tier in ("css-official", "reliable-cr", "fairly-stable", "rough-interop"):
        heading = next(n for n in nodes["css"].values() if n.attrs.get("id") == tier)
        for sibling in heading.parent.children[heading.parent.children.index(heading) + 1:]:
            if not isinstance(sibling, assets.Node):
                continue
            if sibling.tag in ("h1", "h2", "h3", "h4", "h5", "h6") and sibling.tag <= heading.tag:
                break
            expected_modules.update((tier, n.path()) for n in sibling.walk() if n.tag == "dt")
    actual_modules = {(r["tierAnchor"], r["sourceDOM"][0]) for r in supporting["cssModuleEntries"]}
    if actual_modules != expected_modules or len(actual_modules) != len(supporting["cssModuleEntries"]):
        raise ValueError("missing/duplicate/CSS-tier-drift module review")
    for record in supporting["cssModuleEntries"]:
        excerpt = " — ".join(assets.normalized(nodes["css"][p]) for p in record["sourceDOM"])
        if (excerpt != record["excerpt"] or assets.sha(excerpt.encode()) != record["excerptUTF8SHA256"] or
                record["sourceHash"] != entries["css"]["sha256"]):
            raise ValueError("CSS module source excerpt drift")
        if record["featureOrSupportDeclaration"] or any(v != "not-tested" for v in record["capabilities"].values()):
            raise ValueError("CSS module source review must not invent feature evidence")
        if any(not record.get(k) for k in ("plannedPhase", "applicability", "EPUBIncorporation")):
            raise ValueError("CSS module lacks applicability/stage/incorporation")

    draft = json.loads((root / "reviews/official-cases.json").read_text())
    index = json.loads((root / "official-tests/index.json").read_text())
    cases = {r["id"]: r for r in index["cases"]}
    if len(draft["cases"]) != len(cases) or {r["id"] for r in draft["cases"]} != set(cases):
        raise ValueError("missing/duplicate official semantic case")
    amendments = json.loads((root / "reviews/official-anchor-amendments.json").read_text())
    if any(entries[d]["sha256"] != h for d, h in amendments["sourceHashes"].items()):
        raise ValueError("official correspondence fixed source drift")
    reviewed = {}
    for amendment in amendments["mappings"]:
        for case in amendment["cases"]:
            if case not in cases or case in reviewed:
                raise ValueError("unknown/duplicate case correspondence")
            reviewed[case] = amendment

    def correspondence(document, node):
        path = node.path()
        linked = [r for r in matrix["rows"] if r["document"] == document and
                  (r["domPath"] == path or r["domPath"].startswith(path + "/") or path.startswith(r["domPath"] + "/"))]
        if not linked:
            raise ValueError(f"source correspondence has no matrix clause: {document} {path}")
        return {"document": document, "fixedREC": entries[document]["requestUrl"],
                "sourceHash": entries[document]["sha256"], "sourceDOM": path,
                "section": node.section(), "excerpt": assets.normalized(node),
                "excerptSHA256": assets.sha(assets.normalized(node).encode()),
                "clauses": [{k: r[k] for k in ("featureId", "normativeLevel", "excerpt", "excerptSHA256", "applicability", "phase", "decision", "reason")} for r in linked]}

    backlinks, by_case, external_links = [], {}, []
    for document in ("epub", "rs", "a11y"):
        for node in nodes[document].values():
            for raw in node.attrs.get("data-tests", "").split(","):
                reference = raw.strip()
                if not reference:
                    continue
                source = {"document": document, "fixedREC": entries[document]["requestUrl"],
                          "sourceHash": entries[document]["sha256"], "sourceDOM": node.path(),
                          "sourceAttribute": node.attrs["data-tests"], "reference": reference,
                          "sourceExcerpt": assets.normalized(node),
                          "sourceExcerptSHA256": assets.sha(assets.normalized(node).encode())}
                if not reference.startswith("#"):
                    external_links.append({**source, "status": "external-link; not-executed",
                                           "reason": "External structural-test reference, not a frozen official RS fixture"})
                    continue
                case = reference[1:]
                if case in cases:
                    context = correspondence(document, clause_context(node))
                    by_case.setdefault(case, []).append(context)
                    backlinks.append({**source, "caseId": case, "status": "known-case; source-mapped; not-executed",
                                      "clauseContextDOM": context["sourceDOM"]})
                else:
                    backlinks.append({**source, "caseId": case, "status": "unresolved; not-executed",
                                      "reason": "Fragment absent from the frozen 169-case report/source inventory; no rename, edition substitution or equivalence established"})

    output_cases = []
    for record in draft["cases"]:
        original = cases[record["id"]]
        if (record["executed"] or record["result"] != "not-tested" or
                record["reportIdentity"]["expectedStatement"] != original["expected"] or
                record["reportIdentity"]["references"] != original["reportReferences"] or
                record["reportIdentity"]["normativeLevel"] != original["normativeLevel"] or
                record["sourceIdentity"]["archiveSHA256"] != original["sourceArchiveSHA256"] or
                record["artifactIdentity"]["generatedSHA256"] != original["generatedSHA256"]):
            raise ValueError("case review changed history/source identity or invented execution")
        links, missing = [], []
        for target in record["targets"]:
            document = target["document"]
            fragment = urlsplit(target["reportURL"]).fragment
            node = next((n for n in nodes[document].values() if n.attrs.get("id") == fragment), None)
            if node is None:
                missing.append(target["reportURL"])
                continue
            links.append(correspondence(document, clause_context(node)))
        amendment = reviewed.get(record["id"])
        if missing and not amendment:
            raise ValueError(f"obsolete report anchor without reviewed correspondence: {record['id']}")
        if amendment:
            for target in amendment["targets"]:
                links.append(correspondence(target["document"], nodes[target["document"]][target["domPath"]]))
        existing = {(link["document"], link["sourceDOM"]) for link in links}
        for link in by_case.get(record["id"], []):
            identity = (link["document"], link["sourceDOM"])
            if identity not in existing:
                links.append(link)
                existing.add(identity)
        if not links or not record["plannedStages"] or not record["applicability"]:
            raise ValueError("case review lacks target/stage/applicability")
        output_cases.append({**{k: v for k, v in record.items() if k not in ("targets", "gaps")},
                             "initialDraftTargets": record["targets"], "initialDraftGaps": record["gaps"],
                             "fixedSourceCorrespondences": links,
                             "recBacklinkStatus": "source-mapped; not-executed" if record["id"] in by_case else
                                                  "no-fixed-REC-backlink; original report mapping retained; not-executed",
                             "obsoleteHistoricalReferences": missing,
                             "explicitCorrespondenceReview": amendment,
                             "pairedFixtures": original.get("pairedFixtures", []),
                             "reviewStatus": "source-mapped; not-executed"})
    return {"schemaVersion": 1, "semanticComplete": False,
            "remainingGate": "Independent semantic acceptance; then fixed-tree Factory Droid",
            "officialCounts": official_counts, "supporting": supporting, "cases": output_cases,
            "legacyPrerequisites": legacy,
            "recBacklinks": {"fragments": backlinks, "externalLinks": external_links,
                             "counts": {"fragmentReferences": len(backlinks),
                                        "distinctFragmentIds": len({r["caseId"] for r in backlinks}),
                                        "knownCases": len(by_case), "casesWithoutRECBacklink": len(set(cases) - set(by_case)),
                                        "unresolvedIds": sorted({r["caseId"] for r in backlinks if r["caseId"] not in cases}),
                                        "externalReferences": len(external_links)}},
            "externalReferences": dependency_review(root, entries, inventory, nodes),
            "inputs": {str(p.relative_to(root)): assets.sha(p.read_bytes()) for p in
                       [root / "matrix.json", root / "official-tests/index.json", root / "reviews/supporting.json",
                        root / "reviews/official-cases.json", root / "reviews/official-anchor-amendments.json",
                        root / "external-dependencies.json", root / "reviews/parent-dependencies-original.json",
                        root / "reviews/dependencies-original-input.json", root / "reviews/non-url-references.json",
                        root / "reviews/t3-prerequisites.json"]}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("index", "verify"))
    parser.add_argument("--root", type=assets.Path, default=assets.ROOT)
    args = parser.parse_args()
    try:
        expected = build(args.root)
        path = args.root / "semantic-index.json"
        if args.command == "index":
            assets.write_json(path, expected)
        elif json.loads(path.read_text()) != expected:
            raise ValueError("semantic index drift; rebuild from fixed review inputs")
        print(json.dumps({"cases": len(expected["cases"]), "supportingSections": len(expected["supporting"]["sectionReviews"]),
                          "cssModules": len(expected["supporting"]["cssModuleEntries"]), "semanticComplete": False}, sort_keys=True))
    except (ValueError, OSError, KeyError) as exc:
        parser.exit(1, f"ERROR: {exc}\n")


if __name__ == "__main__":
    main()
