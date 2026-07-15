#!/usr/bin/env python3
"""Generate WE_UNIT_FIELDS from unitmetadata.slk + WorldEditStrings.txt."""
import re
from pathlib import Path

SLK = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\war3.w3mod\units\unitmetadata.slk")
STRINGS = Path(r"d:\Projects\war3mpq-extractor\extracted-mpq\UI\WorldEditStrings.txt")
OUT = Path(__file__).resolve().parent.parent / "src" / "lib" / "war3" / "we-unit-fields.ts"


def parse_strings(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("[") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        out[key.strip()] = value.strip().strip('"')
    return out


def parse_slk(text: str) -> list[tuple[str, str, str]]:
    """Return (code, displayNameKey, fieldName) rows from SLK, skipping header row Y1."""
    rows_by_y: dict[int, dict[int, str]] = {}
    current_y: int | None = None
    current: dict[int, str] = {}

    def flush_row() -> None:
        nonlocal current_y, current
        if current_y is not None and current_y != 1:
            rows_by_y[current_y] = dict(current)

    for line in text.splitlines():
        m = re.match(r'C;X(\d+);(?:Y(\d+);)?K"?([^"]*)"?', line)
        if not m:
            continue
        x = int(m.group(1))
        if m.group(2):
            flush_row()
            current_y = int(m.group(2))
            current = {}
        if current_y is None:
            continue
        current[x] = m.group(3)

    flush_row()

    result: list[tuple[str, str, str]] = []
    for y in sorted(rows_by_y):
        row = rows_by_y[y]
        code = row.get(1, "")
        display_key = row.get(6, "")
        field_name = row.get(2, "")
        if code and display_key:
            result.append((code, display_key, field_name))
    return result


def to_ts_key(label: str, code: str) -> str:
    """HumanLabel_code — sanitize label to valid TS identifier."""
    # Keep letters, digits, spaces, hyphens; collapse whitespace
    cleaned = re.sub(r"[^\w\s-]", "", label, flags=re.UNICODE)
    cleaned = re.sub(r"\s+", " ", cleaned).strip()
    # Replace spaces/hyphens with nothing for identifier (Tooltip - Basic -> TooltipBasic)
    ident_label = re.sub(r"[\s-]+", "", cleaned)
    if not ident_label:
        ident_label = code
    if ident_label[0].isdigit():
        ident_label = f"_{ident_label}"
    return f"{ident_label}_{code}"


def field_to_label(field_name: str) -> str:
    """Split camelCase field name into words."""
    parts = re.sub(r"([a-z])([A-Z])", r"\1 \2", field_name)
    parts = re.sub(r"(\d+)", r" \1", parts)
    return parts.strip().title()


def main() -> None:
    strings = parse_strings(STRINGS.read_text(encoding="utf-8", errors="replace"))
    rows = parse_slk(SLK.read_text(encoding="utf-8", errors="replace"))

    entries: list[tuple[str, str, str]] = []
    used_keys: dict[str, int] = {}

    for code, display_key, field_name in rows:
        label = strings.get(display_key) or field_to_label(field_name) or display_key
        key = to_ts_key(label, code)
        if key in used_keys:
            used_keys[key] += 1
            key = f"{key}_{used_keys[key]}"
        else:
            used_keys[key] = 0
        entries.append((key, code, label))

    OUT.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        "// Auto-generated from unitmetadata.slk + WorldEditStrings.txt",
        "// Regenerate: python scripts/gen-we-unit-fields.py",
        "",
        "export const WE_UNIT_FIELDS = {",
    ]
    for key, code, _ in entries:
        lines.append(f'  {key}: "{code}",')
    lines.append("} as const;")
    lines.append("")
    OUT.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {len(entries)} fields to {OUT}")


if __name__ == "__main__":
    main()
