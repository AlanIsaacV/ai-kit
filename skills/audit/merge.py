#!/usr/bin/env python3

import json
import re
import sys

FOCUS_AXES = {"1": (1,), "2": (2,), "3-4": (3, 4), "5-6": (5, 6)}
FOCUS_ORDER = {"1": 0, "2": 1, "3-4": 2, "5-6": 3}
SEVERITY_RANK = {"CRITICAL": 0, "HIGH": 1, "MEDIUM": 2, "NEEDS REVIEW": 3}
KEPT_FIELDS = ("severity", "confidence", "title", "exposure_path", "missing_control", "exploit", "would_confirm")
ITEM = re.compile(r"^([0-9]+)\.([0-9]+)$")
ANCHOR = re.compile(r"^(.+):([0-9]+)(?:-([0-9]+))?$")


class Invalid(Exception):
    def __init__(self, code, error):
        super().__init__(error)
        self.code = code
        self.error = error


def item_key(item):
    match = ITEM.match(item)
    return int(match.group(1)), int(match.group(2))


def parse_anchor(where, anchor):
    match = ANCHOR.match(anchor) if isinstance(anchor, str) else None
    if match is None:
        raise Invalid("anchor-invalid", f"{where}: anchor {json.dumps(anchor)} is not path:line or path:first-last")
    first = int(match.group(2))
    last = int(match.group(3)) if match.group(3) is not None else first
    if first < 1 or last < first:
        raise Invalid("anchor-invalid", f"{where}: anchor {json.dumps(anchor)} has no valid line range")
    return match.group(1), first, last


def format_anchor(anchor):
    path, first, last = anchor
    return f"{path}:{first}" if first == last else f"{path}:{first}-{last}"


def read_lens(path):
    try:
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
    except FileNotFoundError:
        raise Invalid("lens-missing", f"{path}: lens file not found; write the lens's JSON answer there first")
    except OSError as error:
        raise Invalid("lens-unreadable", f"{path}: lens file cannot be read ({error.strerror})")
    except UnicodeDecodeError:
        raise Invalid("lens-not-json", f"{path}: lens file is not UTF-8 text")
    try:
        lens = json.loads(text)
    except json.JSONDecodeError as error:
        raise Invalid("lens-not-json", f"{path}: lens file is not valid JSON ({error.msg} at line {error.lineno}, column {error.colno})")
    if not isinstance(lens, dict):
        raise Invalid("lens-invalid", f"{path}: lens file must hold one JSON object, the lens's whole answer")
    focus = lens.get("focus")
    if focus not in FOCUS_AXES:
        raise Invalid("lens-invalid", f"{path}: focus must be one of 1, 2, 3-4, 5-6; found {json.dumps(focus)}")
    findings = lens.get("findings")
    if not isinstance(findings, list):
        raise Invalid("lens-invalid", f"{path}: findings must be a list, empty when the lens found nothing")
    return focus, [read_finding(f"{path}: findings[{index}]", focus, finding) for index, finding in enumerate(findings)]


def read_finding(where, focus, finding):
    if not isinstance(finding, dict):
        raise Invalid("finding-invalid", f"{where}: a finding must be a JSON object")
    items = finding.get("items")
    if not isinstance(items, list) or not items:
        raise Invalid("finding-invalid", f"{where}: items must be a non-empty list of checklist numbers")
    for item in items:
        if not isinstance(item, str) or not ITEM.match(item):
            raise Invalid("finding-invalid", f"{where}: item {json.dumps(item)} is not a checklist number such as 2.5")
    primary = finding.get("primary_item")
    if primary is None:
        raise Invalid("primary-item-missing", f"{where}: primary_item is missing; it must name one of the finding's items within focus {focus}")
    if primary not in items:
        raise Invalid("primary-item-not-in-items", f"{where}: primary_item {json.dumps(primary)} is not among its items {json.dumps(items)}")
    if item_key(primary)[0] not in FOCUS_AXES[focus]:
        raise Invalid("primary-item-outside-focus", f"{where}: primary_item {json.dumps(primary)} is outside the lens's focus {focus}")
    if finding.get("severity") not in SEVERITY_RANK:
        raise Invalid("finding-invalid", f"{where}: severity must be CRITICAL, HIGH, MEDIUM or NEEDS REVIEW; found {json.dumps(finding.get('severity'))}")
    anchors = finding.get("anchors")
    if not isinstance(anchors, list) or not anchors:
        raise Invalid("finding-invalid", f"{where}: anchors must be a non-empty list; a finding left with no anchor is dropped before the merge")
    return {
        "primary_item": primary,
        "items": list(items),
        "anchors": [parse_anchor(where, anchor) for anchor in anchors],
        "fields": {name: finding.get(name) for name in KEPT_FIELDS},
    }


def overlaps(left, right):
    return any(
        a[0] == b[0] and a[1] <= b[2] and b[1] <= a[2]
        for a in left["anchors"]
        for b in right["anchors"]
    )


def absorb(anchors):
    unique = list(dict.fromkeys(anchors))
    return [
        anchor
        for anchor in unique
        if not any(
            other != anchor and other[0] == anchor[0] and other[1] <= anchor[1] and anchor[2] <= other[2]
            for other in unique
        )
    ]


def merge(lenses):
    findings = [
        dict(finding, focus=focus, order=(FOCUS_ORDER[focus], index))
        for focus, lens_findings in lenses
        for index, finding in enumerate(lens_findings)
    ]
    parent = list(range(len(findings)))

    def root(node):
        while parent[node] != node:
            parent[node] = parent[parent[node]]
            node = parent[node]
        return node

    for i, left in enumerate(findings):
        for j in range(i + 1, len(findings)):
            right = findings[j]
            if left["primary_item"] == right["primary_item"] and overlaps(left, right):
                parent[root(j)] = root(i)

    groups = {}
    for index, finding in enumerate(findings):
        groups.setdefault(root(index), []).append(finding)

    merged = []
    for members in groups.values():
        members.sort(key=lambda finding: (SEVERITY_RANK[finding["fields"]["severity"]], finding["order"]))
        kept = members[0]
        anchors = [anchor for member in members for anchor in member["anchors"]]
        items = sorted({item for member in members for item in member["items"]}, key=item_key)
        lenses_seen = sorted({member["focus"] for member in members}, key=FOCUS_ORDER.get)
        result = {"primary_item": kept["primary_item"], "items": items}
        result.update({name: value for name, value in kept["fields"].items() if value is not None})
        result["anchors"] = [format_anchor(anchor) for anchor in absorb(anchors)]
        result["lenses"] = lenses_seen
        merged.append((kept, result))

    merged.sort(key=lambda pair: (SEVERITY_RANK[pair[1]["severity"]], item_key(pair[1]["primary_item"]), pair[0]["order"]))
    return [result for _, result in merged]


def parse_args(argv):
    paths = []
    flags_done = False
    for argument in argv:
        if not flags_done and argument == "--":
            flags_done = True
        elif not flags_done and argument.startswith("-"):
            raise Invalid("unknown-flag", f"{argument}: merge.py takes no flags; pass the lens JSON files as arguments, and put a path that begins with a dash after --")
        else:
            paths.append(argument)
    if not paths:
        raise Invalid("usage", "no lens file given; pass the lens JSON files as arguments: merge.py <lens-1.json> <lens-2.json> <lens-3-4.json> <lens-5-6.json>")
    return paths


def main(argv):
    try:
        paths = parse_args(argv)
        lenses = []
        seen = {}
        for path in paths:
            focus, findings = read_lens(path)
            if focus in seen:
                raise Invalid("duplicate-focus", f"{path}: focus {focus} was already given by {seen[focus]}; pass one file per lens")
            seen[focus] = path
            lenses.append((focus, findings))
    except Invalid as invalid:
        print(json.dumps({"error": invalid.error, "code": invalid.code}, ensure_ascii=False))
        return 2
    findings = merge(lenses)
    answer = {
        "interpreted": {"lenses": [{"file": path, "focus": focus, "findings": len(found)} for path, (focus, found) in zip(paths, lenses)]},
        "count": len(findings),
        "findings": findings,
    }
    print(json.dumps(answer, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
