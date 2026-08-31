#!/usr/bin/env python3
"""Validate the Grafana dashboards before they are shipped to a server.

These files are mounted read-only into Grafana and reloaded from disk every 30
seconds, which is convenient and unforgiving: Grafana logs a malformed file and
carries on serving the previous version, so a broken dashboard looks exactly
like a dashboard nobody has changed. Nothing turns red. That is why this runs
in CI rather than on the server.

The panel rule below is not a style preference. An instant query reads a single
moment — the end of the window — so pairing it with $__rate_interval answers
"what happened in the last minute" no matter which range the operator picked,
and on a quiet minute it renders 0 for every row instead of admitting it has
nothing to say. Both API tables shipped that way (#346).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "monitoring" / "grafana" / "dashboards"


def panels(node: dict):
    """Yield every panel, including the ones nested inside collapsed rows."""
    for panel in node.get("panels", []) or []:
        if panel.get("type") == "row":
            yield from panels(panel)
        else:
            yield panel


def main() -> int:
    files = sorted(ROOT.glob("*.json"))
    if not files:
        print(f"no dashboards found under {ROOT}", file=sys.stderr)
        return 1

    problems: list[str] = []
    for path in files:
        try:
            dashboard = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            problems.append(f"{path.name}: not valid JSON — {exc}")
            continue

        if not dashboard.get("uid"):
            problems.append(f"{path.name}: no uid, so provisioning cannot keep it stable")

        for panel in panels(dashboard):
            title = panel.get("title", "(untitled)")
            for target in panel.get("targets", []) or []:
                expr = target.get("expr", "")
                if target.get("instant") and "$__rate_interval" in expr:
                    problems.append(
                        f"{path.name}: panel {title!r} pairs an instant query with "
                        f"$__rate_interval — it will only ever read the last scrape "
                        f"interval. Use increase(...[$__range]) for a total over the "
                        f"selected window."
                    )

    for problem in problems:
        print(f"  {problem}")
    print(f"checked {len(files)} dashboards, {len(problems)} problems")
    return 1 if problems else 0


if __name__ == "__main__":
    raise SystemExit(main())
