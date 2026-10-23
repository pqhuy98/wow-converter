# Animation export

## Optimization

`POST /api/export/character` accepts `optimization.freezeHead`. It defaults to true in the API, Character Export and Browse UI. **Freeze head outside portraits** can be disabled independently of other export settings.

[Character Export control](../screenshots/ui/export-head-freeze.png) · [Browse settings](../screenshots/ui/browse-head-freeze.png).

When enabled, descendants of the `Head` bone keep a fixed pose outside `Portrait*` sequences. `Head` itself remains animated so head turns are retained. Portrait motion remains intact. Independent global tracks and nonlinear tracks are skipped.

Conservative key reduction always runs for final exports. It removes redundant keys from linear translation, rotation and scale tracks while retaining sequence endpoints. The permitted interpolation differences are 0.05 model units per translation component, 0.1° quaternion angle and 0.0001 per scale component. There is no separate toggle. This optimizes animation data; it does not reduce mesh geometry or texture resolution.

## Map NPC policy

Map generation additionally removes mount/sleep animations using both the WC3 name and original WoW source identities, including mounted poses mapped to ordinary names such as `Stand Alternate`. It deduplicates practically identical sequences within each exact WC3 animation-name family, then retains at most four variants per family. A family ignores only its generated numeric variant suffix: `Attack` and `Attack Slam` remain separate families.

Deduplication requires matching duration, looping and movement speed. It compares every non-global channel at the union of both curves' key times and the segment midpoints, and requires matching event offsets. Similarity limits are 0.05 model units per translation component, 0.1° rotation, 0.001 per scale component and 0.0001 for other numeric channels. Nonlinear tracks require exact keys and tangents. Independent global tracks share their existing clock. For example, the comparison demo's Mulgris `Attack 1` and `Attack 2` have identical bone curves and duration and collapse into one map-export sequence. Individual Character/Browse exports retain their sequence selection and do not apply this map-only variant policy.

The map NPC appearance-cache fingerprint includes this policy, so an older untrimmed cached model cannot bypass it.

## Selection and renaming

Character export requests accept `character.sequenceSelection` (source record indices scoped to a model/build) and `animationRenames`: a list of `{wowName, wowVariant, newWc3Name}`. Renames apply to the base output after sorting and generated-animation processing, preserving sequence order. Overridden names serialize exactly, without a numeric suffix; names are limited to 79 UTF-8 bytes and cannot contain quotes, backslashes, line breaks or NUL. Conflicting renames for source records merged into one combined sequence fail explicitly.

Animation-only re-export passes the current output's `modelScale` to preserve scale. Export Character uses the current form settings and offers Keep changes or Discard for non-default animation edits when the base reference and build match the last successful export. Keep changes carries selection and renames, including drafts, without retaining the old scale override.
