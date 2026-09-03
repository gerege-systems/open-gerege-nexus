#!/usr/bin/env python3
"""Generate Presentation/Localization/L10nCatalog.swift from the Windows client's
Resources.resw files, keeping the macOS string keys in parity with Windows.

Usage (from desktop-app/macos-app):
    python3 scripts/gen_l10n.py
"""
import json
import xml.etree.ElementTree as ET
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent          # macos-app/
STRINGS = HERE.parent / "windows/src/NexusGeregeDesktop.Client/Strings"
OUT = HERE / "Presentation/Localization/L10nCatalog.swift"

# Swift catalog property name -> .resw culture folder. Key order follows en-US.
CULTURES = {"en": "en-US", "mn": "mn-MN", "ru": "ru-RU", "zh": "zh-CN",
            "fr": "fr-FR", "es": "es-ES", "ar": "ar-SA"}


def load(path: Path) -> dict[str, str]:
    # Build-time script parsing the repo's own trusted .resw files — no
    # untrusted/network XML input, so there is no XXE attack surface.
    tree = ET.parse(path)  # nosemgrep: python.lang.security.use-defused-xml-parse.use-defused-xml-parse
    out: dict[str, str] = {}
    for data in tree.getroot().findall("data"):
        name = data.get("name")
        value = data.find("value")
        if name and value is not None:
            out[name] = value.text or ""
    return out


def esc(s: str) -> str:
    # JSON string escaping is a valid Swift string literal for our content
    # (no \uXXXX sequences because the text is emitted verbatim).
    return json.dumps(s, ensure_ascii=False)


def main() -> None:
    tables = {name: load(STRINGS / f"{culture}/Resources.resw") for name, culture in CULTURES.items()}
    keys = list(tables["en"].keys())

    lines = [
        "// AUTO-GENERATED from windows-app Resources.resw — do not edit by hand.",
        "// Regenerate: python3 scripts/gen_l10n.py (key parity with the Windows client).",
        "",
        "enum L10nCatalog {",
    ]
    for i, name in enumerate(CULTURES):
        if i:
            lines.append("")
        lines.append(f"    static let {name}: [String: String] = [")
        lines += [f"        {esc(k)}: {esc(tables[name].get(k, ''))}," for k in keys]
        lines.append("    ]")
    lines += ["}", ""]

    OUT.write_text("\n".join(lines))
    print(f"wrote {len(keys)} keys x {len(CULTURES)} locales -> {OUT.relative_to(HERE)}")


if __name__ == "__main__":
    main()
