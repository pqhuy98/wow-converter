# NPC unit export

Map generation fills unit WE fields from the bundled AzerothCore world data and
the exported creature model. Ownership remains Player 1. NPC icons are generated
from the exported model; unit sound sets are not overridden. Explicit
NPC-as-doodad mode remains decoration only.

## Model animations

NPCs without equipment default to the Unarmed attack family. Equipped NPCs infer their family from the exported weapon inventory types; other weapon-style attacks are filtered before writing the model.

Map generation enables freezing head descendants outside portraits and always applies conservative key reduction. It also removes Mount/Sleep animations, deduplicates practically identical variants within each WC3 animation-name family, then keeps at most four variants per family. Stand/Attack/Spell families remain distinct, including tags such as Attack Slam. [Animation export](../rendering/animations.md) defines the option, tolerances and map-only scope.

## Source combat stats

Maximum HP/mana and melee/ranged damage are exported at their source
values, rounded to WC3 integer fields. They are independent of which other NPCs
are selected; there is no WC3 reference-range normalization. Damage dice preserve
both rounded source endpoints, and attack 2 remains disabled. Zero mana/armor
remain zero, and noncombat creatures have no attacks.

Armor is converted to preserve armor-only physical effective HP against an
equal-level attacker, using AzerothCore's `Unit::CalcArmorReducedDamage` formula.
For attacker level L, K is `400 + 85*L` through level 59, or
`400 + 85*(L + 4.5*(L-59))` above 59. Source reduction is
`min(0.75, armor/(armor+K))`; physical effective HP is `HP/(1-reduction)`.
Stock WC3 uses `0.06*armor/(1+0.06*armor)`, so exported armor is
`round(min(3, sourceArmor/K)/0.06)`, capped at 50. HP itself stays unchanged.
The fixed armor field uses the NPC's source maximum level as the reference
attacker level. Different attacker levels, armor penetration, buffs, WC3 attack
versus defense-type multipliers and custom armor constants are outside this
approximation. Integer armor introduces at most 0.03 error in the EHP/HP multiplier.

Source values use class/level/expansion base stats and template modifiers,
following AzerothCore's default-rate formulas. Spawn `curhealth` and `curmana`
remain source metadata rather than maximums. WC3 units start with full source
health and mana. Source server-rate changes, scripted buffs and encounter-specific
scaling are not evaluated.

## Field heuristics

- Model and portrait share the exported model. Collision uses the enclosing
  horizontal circle of the source M2 collision box, centered on the unit origin.
  Missing, empty or invalid boxes fall back to the enclosing horizontal circle
  of vertices in the lowest 25% of the exported model height. Both use map fit,
  display scale and export unit size once, retaining the 8..1024 WC3 radius
  limits (8 for empty geometry). Server BoundingRadius is no longer used.
  Collision metadata refreshes even when models are reused without Fresh export.
  Combat reach determines melee reach. Selection circles independently follow
  the unmounted WotLK client calculation using CreatureModelData GeoBox bounds,
  display/model/racial/family scaling and source object scale. The native radius
  is converted to WC3 distance, including map fit and export unit size once;
  WE Selection Scale is the resulting diameter divided by 72. The old .5 floor
  and collision-based heuristic are removed. Unit sizing changes apply on top
  of native WoW scaling without applying the latter a second time. The client
  lookup tables are cached until the source build changes. Missing/empty native
  stand boxes use WoW's own 1.2-yard fallback; unsupported data schemas raise an
  error. This models static unmounted NPCs; temporary auras/mount state and WoW's
  circle texture/pulse visuals are not reproduced. Collision remains independent
  of the model's embedded picking geometry.
- Humanoids stay upright; beasts can pitch/roll slightly. Death time covers the
  longest Death animation. Required alternate tags are set only when the model
  has alternate Stand sequences and no ordinary Stand sequence.
- Movement uses source ground/swim/flight/rooted capabilities. Dual ground/flight
  actors conservatively retain ground movement; swim-capable ground creatures
  use amphibious movement. Speed is source yards/second times terrain units per
  yard, independent of the visual Unit scale setting, and capped at 522. Ground
  actors use source run speed; walking patrol state is not reproduced. Existing
  placement eligibility still excludes creatures outside playable heights or too
  far from fitted ground.
- HP regeneration uses 0.1% of source HP/second when source regeneration is
  enabled; mana uses 0.2%/second. Native regeneration is an approximation, not
  AzerothCore's out-of-combat regeneration state machine.
- Undead, mechanical and giant classifications are mapped where native WC3
  semantics exist. Race/campaign fields organize exported types in WE. Rank does
  not automatically turn an NPC into a WC3 hero.
- Defense type uses converted armor, caster class and flight; mechanical bodies
  use metal armor sounds. Nonphysical damage schools map to WC3 magic attacks.
  WoW school resistances and WC3 armor tables are not interchangeable.
- Recognized ranged equipment selects arrow, thrown axe, wand or rifle behavior.
  Stock projectile paths are verified assets. Rifles deliberately use WC3's
  instant impact attack; other ranged weapons use traveling missiles. Range,
  speed, straight arc and homing are conservative native defaults.
- Model attack sound events provide a release-time cue where available; otherwise
  use 35% of the attack animation. Timing is bounded by the source cooldown.
  Embedded attack sounds suppress an extra native weapon-impact sound. Otherwise
  equipment subclass/body family selects a WC3 weapon sound. Weapon/hand
  attachments supply launch offsets, with a body-based fallback.
- Critters, noncombat pets, pacified actors and zero-damage creatures do not
  inherit attacks. Civilian flags suppress automatic acquisition. Gossip/vendor
  flags alone never disable combat. Inherited abilities, upgrades, training,
  building commands and melee resource costs are cleared explicitly.

WoW spells, vendors, quests, patrols and encounter AI are not translated into
runtime gameplay by these unit-field mappings.

## Caching and audit

NPC appearance metadata is resolved before model conversion. The deduplication
key includes the loaded build, resolved base model, skin textures, character
customizations/geosets, particle colors, equipment and model export settings;
creature/template/display identity alone does not force another model export.
Converted and reused cached models also share output when their serialized
content matches, ignoring only the model-name field. Display scale is excluded
for particle-free creatures because it does not affect their model conversion.
Gameplay stats, ownership and placement remain independent for each unit type.

Shared models live at `npcs/<normalized-wow-name>.mdx`. Names use lowercase ASCII
letters, digits and dashes. Different appearances with the same normalized name
receive `_<display ID>` suffixes. The representative name is selected
deterministically; appearance aliases use the same model path. `npcs/models.json`
stores the resolved configuration mapping. Fresh export off reuses models with
a matching current appearance/animation-policy fingerprint. Unversioned legacy
root-level `creature-<id>.mdx` models are rebuilt, then removed after successful
export; they cannot bypass attack filtering or the current animation policy.

Unit `upor` and NPC destructible `bgpm` use the same shared model path as their
model field, including cached exports. NPC-as-destructibles mode skips icon
rendering and icon-cache updates entirely.

In unit mode, one icon per shared model uses its portrait camera, a frozen Stand pose and
1.7x framing. Three tabs reuse one installed headless browser, capturing at 384px
and producing 128px ClassicHD2.0 BTN/DISBTN BLPs. The active icon is
`ReplaceableTextures/CommandButtons/BTN_<model-name>.blp`; the matching DISBTN
lives under `ReplaceableTextures/CommandButtonsDisabled/`. Unit `uico` points to
the shared active icon. `npcs/icons.json` caches icons against model and texture
contents plus rendering settings. Fresh export rebuilds them. Missing browser,
viewer, model camera, capture or icon encoding errors retain the default Footman
icon and do not fail map generation; explicit job cancellation still stops work.

Existing creature models are reused with Fresh export off. A small metadata scan
seeks past geometry/textures and decodes only animation/event/attachment chunks.
`units.json` records original source identities and gameplay stats, placed type
codes, exclusion reasons and shared model/icon paths.
Type-wide editor suffixes no longer contain a misleading first-spawn GUID.

The bundled SQLite includes expanded creature/item templates, class-level stats,
model radius/reach, template movement and spawn movement overrides. Regenerate
these through the existing `generate:acore-sqlite` pipeline. Older bundles without
the expanded template schema use conservative level-based defaults; an incomplete
expanded bundle raises an error rather than silently losing NPCs.

Unit fields are regenerated on the next map generation. Object-data changes need
a restarted WC3 map session. Offline tests cover source calculations, preserved
combat stats, stock-field cleanup and cached model metadata. Native animation, pathing,
projectile and sound behavior still needs in-game checking.
