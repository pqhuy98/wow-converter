#!/usr/bin/env python3
"""Generate WE_UNIT_FIELDS from unitmetadata.slk + WorldEditStrings.txt."""
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SLK = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\war3.w3mod\units\unitmetadata.slk")
STRINGS = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\UI\WorldEditStrings.txt")
EDITOR_DATA = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\UI\UnitEditorData.txt")
TRIGGER_DATA = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\war3.w3mod\ui\triggerdata.txt")
OUT_TS = ROOT / "src" / "lib" / "war3" / "we-unit-fields.ts"
OUT_GO = ROOT / "golang" / "internal" / "war3" / "we_unit_fields.go"

PRIMITIVES = frozenset({"int", "real", "unreal", "bool", "string", "char"})

# Editor type -> UnitEditorData section when names differ.
EDITOR_SECTION_ALIASES: dict[str, str] = {
    "unitClass": "unitClass",
    "pathingListPrevent": "pathingListPrevent",
    "pathingListRequire": "pathingListRequire",
}

# Types without UnitEditorData sections, or needing extra detail beyond the section.
TYPE_OVERRIDES: dict[str, str] = {
    "icon": "string path (.blp), import Image",
    "model": "string path (.mdl), import Model",
    "modelList": "string list, comma-separated model paths, import Model",
    "pathingTexture": "string path (.blp), import Image",
    "shadowTexture": "string path (.blp), import Image",
    "shadowImage": "string enum, Shadow|ShadowFlyer",
    "soundLabel": "string sound label key",
    "unitSound": "string unit sound set key",
    "uberSplat": "string 4-char ubersplat code",
    "abilCode": "int 4-char rawcode",
    "abilityList": "string list, comma-separated 4-char ability rawcodes",
    "heroAbilityList": "string list, comma-separated 4-char ability rawcodes",
    "abilitySkinList": "string list, comma-separated 4-char ability rawcodes",
    "techList": "string list, comma-separated 4-char upgrade rawcodes",
    "unitList": "string list, comma-separated 4-char unit rawcodes",
    "itemList": "string list, comma-separated 4-char item rawcodes",
    "upgradeList": "string list, comma-separated 4-char upgrade rawcodes",
    "stringList": "string list, comma-separated tokens",
    "intList": "string list, comma-separated integers",
    "tilesetList": "string, *|A|B|C|D|L|N|O|U|Z tileset codes",
}


def parse_strings(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("[") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        out[key.strip()] = value.strip().strip('"')
    return out


def parse_unit_editor_data(text: str) -> dict[str, list[tuple[int, str]]]:
    sections: dict[str, list[tuple[int, str]]] = {}
    current: str | None = None
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("//"):
            continue
        if line.startswith("[") and line.endswith("]"):
            current = line[1:-1]
            sections[current] = []
            continue
        if current is None or line.startswith(("Sort=", "NumValues=")):
            continue
        match = re.match(r"^(\d+)=([^,]+)", line)
        if match:
            sections[current].append((int(match.group(1)), match.group(2)))
    return sections


def parse_trigger_primitives(text: str) -> dict[str, str]:
    primitives: dict[str, str] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("//") or line.startswith("["):
            continue
        match = re.match(r"^([a-z][a-zA-Z0-9]*)=0,0,\d+,WESTRING_[^,]+,(.+)$", line)
        if match:
            primitives[match.group(1)] = match.group(2)
    return primitives


def parse_slk(text: str) -> list[dict[int, str]]:
    rows_by_y: dict[int, dict[int, str]] = {}
    current_y: int | None = None
    current: dict[int, str] = {}

    def flush_row() -> None:
        nonlocal current_y, current
        if current_y is not None and current_y != 1:
            rows_by_y[current_y] = dict(current)

    for line in text.splitlines():
        match = re.match(r'C;X(\d+);(?:Y(\d+);)?K"?([^"]*)"?', line)
        if not match:
            continue
        x = int(match.group(1))
        if match.group(2):
            flush_row()
            current_y = int(match.group(2))
            current = {}
        if current_y is None:
            continue
        current[x] = match.group(3)

    flush_row()
    return [rows_by_y[y] for y in sorted(rows_by_y)]


def field_to_label(field_name: str) -> str:
    parts = re.sub(r"([a-z])([A-Z])", r"\1 \2", field_name)
    parts = re.sub(r"(\d+)", r" \1", parts)
    return parts.strip().title()


def to_ts_key(label: str, code: str) -> str:
    cleaned = re.sub(r"[^\w\s-]", "", label, flags=re.UNICODE)
    cleaned = re.sub(r"\s+", " ", cleaned).strip()
    ident_label = re.sub(r"[\s-]+", "", cleaned)
    if not ident_label:
        ident_label = code
    if ident_label[0].isdigit():
        ident_label = f"_{ident_label}"
    return f"{ident_label}_{code}"


def format_enum_section(
    values: list[tuple[int, str]], primitive: str = "int"
) -> str:
    indices = [index for index, _ in values]
    names = [name for _, name in values]
    lo, hi = min(indices), max(indices)
    range_str = str(lo) if lo == hi else f"{lo}-{hi}"
    return f"{primitive} {range_str}, {'|'.join(names)}"


def format_string_enum_list(values: list[tuple[int, str]]) -> str:
    names = [name for _, name in values]
    return f"string list, {'|'.join(names)}"


def resolve_type_comment(
    editor_type: str,
    editor_sections: dict[str, list[tuple[int, str]]],
    trigger_primitives: dict[str, str],
) -> str | None:
    if editor_type in TYPE_OVERRIDES:
        return TYPE_OVERRIDES[editor_type]

    section_name = EDITOR_SECTION_ALIASES.get(editor_type, editor_type)
    section = editor_sections.get(section_name)
    if section:
        if editor_type.endswith("List") or editor_type in {
            "pathingListPrevent",
            "pathingListRequire",
        }:
            return format_string_enum_list(section)
        primitive = trigger_primitives.get(editor_type.lower(), "int")
        if primitive == "integer":
            primitive = "int"
        return format_enum_section(section, primitive)

    return None


def format_meta_comment(
    row: dict[int, str],
    editor_sections: dict[str, list[tuple[int, str]]],
    trigger_primitives: dict[str, str],
) -> str:
    editor_type = row.get(8, "")
    parts: list[str] = []

    if editor_type in PRIMITIVES:
        parts.append(editor_type)
    elif resolved := resolve_type_comment(editor_type, editor_sections, trigger_primitives):
        parts.append(resolved)
    elif editor_type:
        parts.append(editor_type)

    if min_val := row.get(14):
        parts.append(f"min {min_val}")
    if max_val := row.get(15):
        parts.append(f"max {max_val}")
    if import_type := row.get(10):
        if "import " not in ", ".join(parts):
            parts.append(f"import {import_type}")

    return "// " + ", ".join(parts)


def build_entries(
    rows: list[dict[int, str]],
    strings: dict[str, str],
    editor_sections: dict[str, list[tuple[int, str]]],
    trigger_primitives: dict[str, str],
) -> list[tuple[str, str, str]]:
    entries: list[tuple[str, str, str]] = []
    used_keys: dict[str, int] = {}

    for row in rows:
        code = row.get(1, "")
        display_key = row.get(6, "")
        field_name = row.get(2, "")
        if not code or not display_key:
            continue
        label = strings.get(display_key) or field_to_label(field_name) or display_key
        key = to_ts_key(label, code)
        if key in used_keys:
            used_keys[key] += 1
            key = f"{key}_{used_keys[key]}"
        else:
            used_keys[key] = 0
        entries.append((key, code, format_meta_comment(row, editor_sections, trigger_primitives)))

    return entries


def write_ts(entries: list[tuple[str, str, str]]) -> None:
    OUT_TS.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "// Auto-generated from unitmetadata.slk + WorldEditStrings.txt",
        "// Regenerate: python scripts/gen-we-unit-fields.py",
        "",
        "export const WE_UNIT_FIELDS = {",
    ]
    for key, code, comment in entries:
        lines.append(f'  {key}: "{code}", {comment}')
    lines.append("} as const;")
    lines.append("")
    OUT_TS.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {len(entries)} fields to {OUT_TS}")


def write_go(entries: list[tuple[str, str, str]]) -> None:
    OUT_GO.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "package war3",
        "",
        "// Code generated from unitmetadata.slk + WorldEditStrings.txt; DO NOT EDIT.",
        "// Regenerate: python scripts/gen-we-unit-fields.py",
        "",
        "const (",
    ]
    for key, code, comment in entries:
        lines.append(f'\tWEUnitField_{key} = "{code}" {comment}')
    lines.append(")")
    lines.append("")
    OUT_GO.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {len(entries)} fields to {OUT_GO}")


def main() -> None:
    strings = parse_strings(STRINGS.read_text(encoding="utf-8", errors="replace"))
    editor_sections = parse_unit_editor_data(
        EDITOR_DATA.read_text(encoding="utf-8", errors="replace")
    )
    trigger_primitives = parse_trigger_primitives(
        TRIGGER_DATA.read_text(encoding="utf-8", errors="replace")
    )
    rows = parse_slk(SLK.read_text(encoding="utf-8", errors="replace"))
    entries = build_entries(rows, strings, editor_sections, trigger_primitives)
    write_ts(entries)
    write_go(entries)


if __name__ == "__main__":
    main()
