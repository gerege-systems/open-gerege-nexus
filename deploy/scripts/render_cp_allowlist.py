#!/usr/bin/env python3
"""Render CONTROL_PLANE_ALLOWED_CIDRS as a fail-closed nginx snippet."""

from __future__ import annotations

import ipaddress
import re
import sys


HEADER = (
    "# Managed by the production deploy from CONTROL_PLANE_ALLOWED_CIDRS.\n"
    "# Update the GitHub Actions secret; do not edit this server copy.\n"
)


def parse_networks(raw: str) -> list[str]:
    tokens = [token for token in re.split(r"[\s,]+", raw.strip()) if token]
    if not tokens:
        raise ValueError("no address or CIDR was provided")

    rendered: list[str] = []
    seen: set[str] = set()
    for position, token in enumerate(tokens, start=1):
        try:
            network = ipaddress.ip_network(token, strict=True)
        except ValueError as error:
            raise ValueError(f"token {position} is not a valid address or CIDR") from error

        # A syntactically valid default route is still an invalid console
        # allowlist: it makes the network boundary equivalent to `allow all`.
        if network.prefixlen == 0:
            raise ValueError(f"token {position} permits the entire internet")

        value = str(network.network_address) if "/" not in token else network.with_prefixlen
        if value not in seen:
            rendered.append(value)
            seen.add(value)
    return rendered


def render(raw: str) -> str:
    rules = "".join(f"allow {network};\n" for network in parse_networks(raw))
    return HEADER + rules + "deny all;\n"


def main() -> int:
    try:
        sys.stdout.write(render(sys.stdin.read()))
    except ValueError as error:
        print(f"CONTROL_PLANE_ALLOWED_CIDRS: {error}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
