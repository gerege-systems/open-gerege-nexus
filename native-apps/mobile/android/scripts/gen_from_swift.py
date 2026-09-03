#!/usr/bin/env python3
"""Дизайн ба орчуулгыг ширээний аппаас Android руу ҮҮСГЭНЭ.

iOS нь ширээний Swift файлуудыг ШУУД хуваалцдаг (project.yml → sources).
Android тэгж чадахгүй тул хоёр зүйлийг эндээс үүсгэнэ:

  1. Design/Colors.swift        → ui/theme/EidColors.kt   (өнгөний токенууд)
  2. Presentation/Localization/ → res/values*/eid_strings.xml (7 хэл)

Яагаад үүсгэдэг вэ: гараар хуулсан өнгө, орчуулга хоёр ЧИМЭЭГҮЙ салбарладаг.
Ширээн дээр брэндийн цэнхэр өөрчлөгдөхөд Android дээр хуучин өнгө үлдвэл хэн
ч анзаарахгүй. Энэ скрипт нь тэр зөрүүг боломжгүй болгоно.

    python3 scripts/gen_from_swift.py

Гаралтын файлууд ҮҮСГЭГДСЭН — гараар бүү зас.
"""
import re
import pathlib
import xml.sax.saxutils as sax

HERE = pathlib.Path(__file__).resolve().parent.parent          # native-apps/mobile/android
MAC = HERE.parent.parent / "desktop" / "macos"
COLORS_SWIFT = MAC / "Design" / "Colors.swift"
CATALOG_SWIFT = MAC / "Presentation" / "Localization" / "L10nCatalog.swift"
SERVICE_SWIFT = MAC / "Presentation" / "Localization" / "LocalizationService.swift"

KOTLIN_OUT = HERE / "app/src/main/kotlin/mn/gerege/nexus/ui/theme/EidColors.kt"
RES_DIR = HERE / "app/src/main/res"
# Swift дээрх catalog-ийн property нэр → Android-ийн resource хавтас.
# `mn` нь анхдагч (values/) — энэ бүтээгдэхүүний үндсэн хэл.
LOCALES = {"mn": "values", "en": "values-en", "ru": "values-ru", "zh": "values-zh",
           "fr": "values-fr", "es": "values-es", "ar": "values-ar"}

HEADER = "// ҮҮСГЭСЭН ФАЙЛ — scripts/gen_from_swift.py. Гараар бүү зас.\n// Эх сурвалж: native-apps/desktop/macos/{src}\n"


def gen_colors() -> int:
    src = COLORS_SWIFT.read_text(encoding="utf-8")
    statics = re.findall(r'static let (\w+)\s*=\s*Color\(hex:\s*"([0-9A-Fa-f]{6})"\)', src)
    dynamics = re.findall(r'static let (\w+)\s*=\s*Color\.dynamic\(light:\s*"([0-9A-Fa-f]{6})",\s*dark:\s*"([0-9A-Fa-f]{6})"\)', src)
    lines = [HEADER.format(src="Design/Colors.swift"),
             "package mn.gerege.nexus.ui.theme\n",
             "import androidx.compose.ui.graphics.Color\n",
             "/** Горимоос үл хамаарах брэндийн ramp. */"]
    for name, hex6 in statics:
        lines.append(f"val {name} = Color(0xFF{hex6.upper()})")
    lines.append("\n/** Гэрэл/харанхуйд өөр өөр утгатай токенууд. */")
    lines.append("data class EidColors(")
    for name, _, _ in dynamics:
        lines.append(f"    val {name}: Color,")
    lines.append(")\n")
    for mode, idx in (("Light", 1), ("Dark", 2)):
        lines.append(f"val Eid{mode}Colors = EidColors(")
        for entry in dynamics:
            lines.append(f"    {entry[0]} = Color(0xFF{entry[idx].upper()}),")
        lines.append(")\n")
    KOTLIN_OUT.parent.mkdir(parents=True, exist_ok=True)
    KOTLIN_OUT.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")
    return len(statics) + len(dynamics)


def swift_dicts(text: str) -> dict:
    """`static let mn: [String: String] = [ "Key": "Утга", ... ]` блокуудыг уншина."""
    out = {}
    for lang in LOCALES:
        # Хоёр нэршил: каталогийн `let mn:` ба үйлчилгээний `let extraMn:`
        # (сүүлийнх нь каталогт байхгүй, гараар нэмсэн мөрүүд).
        names = [lang, "extra" + lang.capitalize()]
        for name in names:
            m = re.search(r'(?:static |private static )?let %s\s*:\s*\[String\s*:\s*String\]\s*=\s*\[(.*?)\n\s*\]' % name,
                          text, re.S)
            if not m:
                continue
            pairs = re.findall(r'"((?:[^"\\]|\\.)*)"\s*:\s*"((?:[^"\\]|\\.)*)"', m.group(1))
            out.setdefault(lang, {}).update({k: v for k, v in pairs})
    return out


def gen_strings() -> int:
    merged = {lang: {} for lang in LOCALES}
    for path in (CATALOG_SWIFT, SERVICE_SWIFT):
        for lang, pairs in swift_dicts(path.read_text(encoding="utf-8")).items():
            merged[lang].update(pairs)
    written = 0
    for lang, folder in LOCALES.items():
        pairs = merged.get(lang) or {}
        if not pairs:
            continue
        target = RES_DIR / folder
        target.mkdir(parents=True, exist_ok=True)
        rows = [HEADER.format(src="Presentation/Localization/*.swift").replace("//", "<!--", 1).replace("\n// ", " -->\n<!-- ").rstrip() + " -->",
                '<resources>']
        for key in sorted(pairs):
            value = pairs[key].replace('\\"', '"').replace("\\n", "\n")
            # Android-д ганц хашилт, & нь escape шаарддаг — эс бөгөөс aapt2 унана.
            escaped = sax.escape(value).replace("'", "\\'")
            rows.append(f'    <string name="{key}">{escaped}</string>')
        rows.append('</resources>')
        (target / "eid_strings.xml").write_text("\n".join(rows) + "\n", encoding="utf-8")
        written += len(pairs)
    return written


if __name__ == "__main__":
    print(f"өнгө: {gen_colors()} токен → {KOTLIN_OUT.relative_to(HERE)}")
    print(f"мөр:  {gen_strings()} орчуулга → res/values*/eid_strings.xml")
