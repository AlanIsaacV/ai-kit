import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

MERGE = Path(__file__).resolve().parent / "merge.py"


def finding(primary, items, anchors, severity="HIGH", title=None):
    return {
        "items": items,
        "primary_item": primary,
        "severity": severity,
        "confidence": "confirmed",
        "title": title or f"finding on {primary}",
        "anchors": anchors,
        "exposure_path": "route",
        "missing_control": "guard",
        "exploit": "exploit",
    }


def run_merge(lenses):
    with tempfile.TemporaryDirectory() as directory:
        paths = []
        for focus, findings in lenses.items():
            path = Path(directory) / f"lens-{focus}.json"
            path.write_text(json.dumps({"focus": focus, "findings": findings}), encoding="utf-8")
            paths.append(str(path))
        result = subprocess.run([sys.executable, "-I", str(MERGE), *paths], capture_output=True, text=True)
    if result.returncode != 0:
        raise AssertionError(f"merge.py exited {result.returncode}: {result.stdout}{result.stderr}")
    return json.loads(result.stdout)["findings"]


class MergeRules(unittest.TestCase):
    def test_item_tagged_from_another_axis_does_not_join(self):
        merged = run_merge({
            "1": [finding("1.1", ["1.1"], ["acceso.mjs:312"], severity="CRITICAL")],
            "2": [finding("2.3", ["2.3", "1.1"], ["motions.mjs:211", "acceso.mjs:312"])],
        })
        self.assertEqual(sorted(f["primary_item"] for f in merged), ["1.1", "2.3"])

    def test_anchor_inside_a_range_is_absorbed(self):
        merged = run_merge({
            "2": [
                finding("2.4", ["2.4"], ["lab.mjs:945"]),
                finding("2.4", ["2.4"], ["lab.mjs:945-946"]),
            ],
        })
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["anchors"], ["lab.mjs:945-946"])

    def test_merge_does_not_run_again_over_what_it_merged(self):
        merged = run_merge({
            "2": [
                finding("2.5", ["2.5"], ["lab.mjs:955"]),
                finding("2.5", ["2.5", "6.2"], ["lab.mjs:955"]),
            ],
            "5-6": [finding("6.2", ["6.2"], ["lab.mjs:955"])],
        })
        by_primary = {f["primary_item"]: f for f in merged}
        self.assertEqual(len(merged), 2)
        self.assertEqual(by_primary["2.5"]["lenses"], ["2"])
        self.assertEqual(by_primary["6.2"]["lenses"], ["5-6"])


if __name__ == "__main__":
    unittest.main()
