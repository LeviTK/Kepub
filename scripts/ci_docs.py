"""Conservative PR routing and static local Markdown link checks for Kepub CI."""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import unicodedata
from urllib.parse import unquote, urlsplit


def git(*args):
    return subprocess.check_output(["git", *args])


def changes(base):
    # No rename heuristics: both sides of a rename must be considered. An
    # unrelated base or missing history is an error, never a docs approval.
    git("merge-base", "--is-ancestor", base, "HEAD")
    fields = git("diff", "--no-renames", "--name-status", "-z", base, "HEAD").split(b"\0")
    return [(fields[i].decode("ascii"), fields[i + 1].decode("utf-8"))
            for i in range(0, len(fields) - 1, 2)]


def docs_path(name):
    p = PurePosixPath(name)
    return (name == "README.md" or
            (p.parts[:2] == ("docs", "research") and p.suffix == ".md"
             and p.name != "AGENTS.md"))


def route(event, base):
    if event != "pull_request":
        return "full", []
    changed = changes(base)
    if not changed:
        return "full", changed
    for status, name in changed:
        if status not in ("A", "M") or not docs_path(name):
            return "full", changed
        mode = git("ls-tree", "HEAD", "--", name).split()[0]
        if mode != b"100644":
            return "full", changed
        if status == "M" and git("ls-tree", base, "--", name).split()[0] != b"100644":
            return "full", changed
    return "docs", changed


def markdown(text):
    # Fenced examples are not published links/headings. This is a static
    # repository check, not a Markdown renderer or an external-link crawler.
    lines, fence = [], None
    for line in text.splitlines():
        opening = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if opening:
            marker = opening[1]
            if fence is None:
                fence = marker
            elif marker[0] == fence[0] and len(marker) >= len(fence):
                fence = None
            continue
        if fence is None:
            lines.append(line)
    return "\n".join(lines)


def anchors(text):
    text = markdown(text)
    found = set(re.findall(r'\bid=["\']([^"\']+)["\']', text))
    counts = {}
    for heading in re.findall(r"^ {0,3}#{1,6}\s+(.+?)\s*#*\s*$", text, re.M):
        heading = re.sub(r"\[([^]]+)\]\([^)]+\)", r"\1", heading)
        slug = "".join(c for c in heading.lower()
                       if c in "-_" or not unicodedata.category(c).startswith(("P", "S", "C")))
        slug = slug.replace(" ", "-")
        count = counts.get(slug, 0)
        counts[slug] = count + 1
        found.add(slug + (f"-{count}" if count else ""))
    return found


def links(text):
    text = markdown(text)
    # Inline examples (including HTML snippets) are not rendered links. Match
    # equal-length backtick delimiters, so a longer code span can contain `.
    text = re.sub(r"(?<!`)(`+)(?!`)(.*?)(?<!`)\1(?!`)", "", text, flags=re.S)
    text = re.sub(r"<!--.*?-->", "", text, flags=re.S)
    result = re.findall(r"!?\[[^]\n]*\]\(\s*(<[^>]+>|[^\s)]+)", text)
    # Reference definitions and HTML links/images also carry local targets.
    result += re.findall(r"^ {0,3}\[[^]]+\]:\s*(<[^>]+>|\S+)", text, re.M)
    result += re.findall(r'\b(?:href|src)=["\']([^"\']*)["\']', text)
    return [value.strip("<>") for value in result]


def check(base):
    mode, changed = route("pull_request", base)
    if mode != "docs":
        raise ValueError("docs check requires an eligible nonempty docs-only diff")
    subprocess.run(["git", "diff", "--check", base, "HEAD"], check=True)
    root = Path.cwd().resolve()
    changed_paths = {root / name for _, name in changed}
    checked = 0
    for name in git("ls-files", "-z", "*.md").decode("utf-8").split("\0"):
        if not name:
            continue
        p = root / name
        if p.is_symlink():
            continue
        text = p.read_text(encoding="utf-8")
        if p in changed_paths and re.search(r"^(?:<<<<<<< |=======\s*$|>>>>>>> )", text, re.M):
            raise ValueError(f"{name}: unresolved conflict marker")
        for link in links(text):
            ref = urlsplit(link)
            if ref.scheme or ref.netloc:
                continue
            target = (p.parent / unquote(ref.path)).resolve() if ref.path else p
            # Also catch an unchanged page's link into a removed heading.
            # Unrelated historical broken links do not block this diff.
            if p not in changed_paths and target not in changed_paths:
                continue
            if not target.is_relative_to(root) or not target.exists():
                raise ValueError(f"{name}: missing or outside-repository link: {link}")
            if ref.fragment and target.suffix == ".md":
                if unquote(ref.fragment) not in anchors(target.read_text(encoding="utf-8")):
                    raise ValueError(f"{name}: missing Markdown anchor: {link}")
            checked += 1
    print(json.dumps({"mode": "docs", "files": [p for _, p in changed],
                      "local_links_checked": checked, "product_tests": "not_run"}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("classify", "check"))
    parser.add_argument("--event", default="pull_request")
    parser.add_argument("--base", default="")
    args = parser.parse_args()
    if args.command == "check":
        check(args.base)
        return
    try:
        mode, changed = route(args.event, args.base)
        reason = "allowlisted explanation only" if mode == "docs" else "full validation required"
    except (subprocess.CalledProcessError, UnicodeError, ValueError, IndexError) as error:
        mode, changed, reason = "full", [], f"classification unavailable: {type(error).__name__}"
    print(json.dumps({"mode": mode, "reason": reason, "changes": changed}))
    if output := os.environ.get("GITHUB_OUTPUT"):
        with open(output, "a", encoding="utf-8") as f:
            f.write(f"mode={mode}\n")


if __name__ == "__main__":
    main()
