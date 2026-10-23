# NPC export

The Go exporter uses bundled AzerothCore gameplay data and the exported creature model. Source content is based on Wrath; newer client assets can differ from those spawn locations or display IDs. Owner: `golang/internal/converter/mapexporter/creature/`.

The dialog's **Textures → Unit** setting caps NPC textures at 256px, 512px, 1024px or Unlimited. Classic defaults to 256px; retail defaults to Unlimited. The cap preserves aspect ratio and affects both unit and destructible NPC exports. The API field is `creatures.maxTextureSize`, with zero meaning Unlimited. NPC textures use a separate import namespace so their cap cannot resize scenery textures. Changing the cap invalidates the NPC appearance cache without requiring Fresh export.

## Placement and players

Unit mode excludes spawns outside the playable clamp or too far from fitted ground, retaining them in model export. Ground tolerance is `100 * recommendedClampWidth / chosenClampWidth` WC3 units, so changing the clamp alone does not change the accepted source-space floor gap. **All NPCs as destructibles** places decoration at source heights without pathing footprints; its compatibility key is `allAreDoodads`. Marker colors and placement approximation are in [preview](preview.md).

Player faction appears below the destructible option only when NPC unit export is enabled. Alliance is the default. Slots here are World Editor's one-based numbers:

| Mode | User slot | Race | Name |
|---|---|---|---|
| Alliance | 2, blue | Human | `Player (Alliance)` |
| Horde | 1, red | Orc | `Player (Horde)` |
| NPC export off or all destructibles | 1, red | Human | `Player 1` |

In faction mode, the unselected red/blue slot remains closed/reserved. Named NPC players use slots 3–24. Player names have no diagnostic ID suffixes. Same-name players can retain separate slots when their NPC relationships differ; the preview combines their counts when their reaction toward the user also matches. Every configured player gets a start marker: new user starts are centered, NPC starts occupy a 200-unit grid around the center. Existing markers are preserved; W3I and Lua follow their positions.

`resources/npc-factions.json` contains reviewed ownership policy, Wrath faction templates, masks, flags and explicit friend/enemy exceptions. Named factions retain identity regardless of creature type. Same-name templates merge only if incoming and outgoing hostile/neutral/friendly reactions match against the user and every retained named template in the area. Excluded placements do not consume slots.

Generic factions use only their reaction toward the selected user: hostile becomes Neutral Hostile; neutral/friendly becomes Neutral Passive. When more than 22 named NPC groups remain, the least-populated group moves to neutrals by user reaction and equivalence is recomputed. Ties use name, then lowest template ID. Unknown factions/templates fail export; legacy records with no faction metadata stay with the user.

`factions.json` records assignments, directional reactions and overflow. Static reactions use Human/Orc reference templates and exclude reputation, scripts, temporary overrides and PvP state. Lua follows common-library `SetPlayerAllianceStateBJ` mappings with target-first arguments, named player variables and consecutive opposite directions. Friendly uses `bj_ALLIANCE_ALLIED`, friendly NPC→user uses `bj_ALLIANCE_ALLIED_UNITS` for shared control, neutral uses `bj_ALLIANCE_NEUTRAL`, hostile uses `bj_ALLIANCE_UNALLIED`. W3I/Lua settings and WCT alliance setup persist through editor saves.

## Combat stats

Maximum HP/mana and melee/ranged damage retain source values rounded to WC3 integers, independently of the other selected NPCs. Damage dice preserve both rounded endpoints; attack 2 is disabled. Zero mana/armor stay zero. Stats use class/level/expansion bases and template modifiers at default server rates; spawn `curhealth`/`curmana` remain metadata. Units start at full health/mana. Scripted buffs, encounter scaling and custom server rates are not evaluated.

Armor preserves armor-only physical effective HP against an equal-level attacker using AzerothCore's formula. For level `L`, `K=400+85*L` through 59, then `K=400+85*(L+4.5*(L-59))`. Source reduction is `min(0.75, armor/(armor+K))`; physical EHP is `HP/(1-reduction)`. Stock WC3 reduction uses `0.06*armor/(1+0.06*armor)`, so export writes `round(min(3, sourceArmor/K)/0.06)`, capped at 50, using the NPC's maximum level as reference. HP stays unchanged. Integer armor adds at most 0.03 EHP/HP error; other attacker levels, penetration, buffs, attack/defense multipliers and custom armor constants remain outside this approximation.

## Model-derived and gameplay fields

| Field | Conversion |
|---|---|
| Model/portrait | Unit `umdl`/`upor` and destructible `bfil`/`bgpm` share the resolved model path |
| Collision | Origin-centered horizontal radius of the source M2 collision box; invalid boxes fall back to vertices in the lowest 25% of model height; scale once and clamp to 8–1024 WC3 units |
| Selection | Native unmounted WoW radius from GeoBox bounds and display/model/racial/family/object scaling; convert to WC3 distance, cap to full mesh XY radius, then write diameter/72 |
| Movement | Ground, amphibious, fly or rooted from source capabilities; dual ground/flight conservatively stays ground; source run speed uses terrain units/yard, independently of visual Unit scale, capped at 522 |
| Regeneration | HP 0.1%/second when enabled; mana 0.2%/second; does not reproduce the server's combat-state machine |
| Classification | Native undead/mechanical/giant classifications and editor race/campaign organization; rank does not imply a hero |
| Attacks | Ranged equipment selects verified arrow/axe/wand assets or instant-impact rifles; nonphysical schools use magic attacks; mechanical bodies use metal armor sounds |
| Animation fields | Death time covers the longest Death sequence; alternate tags only when alternate Stand exists without ordinary Stand; humanoids stay upright, beasts permit slight pitch/roll |

Selection uses all geosets around the origin and ignores Z; empty XY geometry keeps the converted WoW radius. Missing/empty native stand boxes use the client's 1.2-yard fallback, while unsupported schemas fail. Collision is independent of embedded picking geometry; its metadata refreshes on cached exports.

Editor race (`urac`) uses the resolved WoW character race: Human/Dwarf/Gnome/Blood Elf/Worgen and related races → Human; Orc/Tauren/Troll/Goblin and related races → Orc; Night Elf/Nightborne → Night Elf; Forsaken and skeleton display races → Undead; Naga → Naga. Fel Orc, Taunka and forest/ice troll display races use Orc. WoW creature types Undead, Demon and Critter/noncombat pet take precedence over the character rig. Explicit humanoid civilians (`flags_extra & 2`) use Commoner, except Undead and Naga retain their race. Known races without a mapped WC3 counterpart (Draenei, Broken, Vrykul, Tuskarr, Pandaren, etc.) use Other. Elemental/mechanical/unspecified/totem/gas-cloud creature types also fall back to Other. Remaining missing-race NPCs retain Creep (`creeps`); None (`unknown`) is not assigned to normal NPCs. Race metadata refreshes even when model files are reused. Campaign (`ucam`) is always Yes for these exported presentation units; it controls editor organization, not race, ownership or campaign membership. Gameplay classifications (`utyp`) continue to follow creature type.

Attack-release timing uses embedded sound events when present, otherwise 35% of the animation, bounded by source cooldown. Embedded attack sounds suppress an extra weapon-impact sound; otherwise equipment/body selects a native sound. Attachments supply launch offsets with a body fallback. Critters, noncombat pets, pacified and zero-damage actors have no inherited attacks; civilian flags suppress acquisition. Gossip/vendor flags alone do not disable combat. Stock abilities, upgrades, training/building commands and melee resource costs are cleared. Spells, vendors, quests, patrols and encounter AI are not translated.

## Shared models and animation policy

NPC units using resolved character models (such as Human or Orc) receive an additional 1.2× multiplier in the unit object's Scaling Value (`usca`), selection circle (`ussc`) and collision size (`ucol`, still capped at 1024). This stacks with their existing sizing, including the selection circle's native/mesh radius cap. Creature models and NPC destructibles retain their previous sizing. Model assets and other object fields are unchanged; cached-model re-exports apply the multiplier too.

Appearance resolution precedes conversion. Keys include build, resolved base model, skins, customizations/geosets, particle colors, equipment and export settings. Identity alone does not force another model. Particle-free display scale is excluded from the model key. Serialized models also deduplicate by content, ignoring the model-name field; gameplay fields, ownership and placement remain independent.

Shared output is `npcs/<normalized-name>.mdx` with lowercase ASCII letters, digits and dashes. Different appearances with colliding names receive `_<display ID>` suffixes. Representative names are deterministic. `npcs/models.json` stores appearance mappings; Fresh export off reuses matching animation-policy/configuration fingerprints. Unversioned root `creature-<id>.mdx` files are rebuilt and cleaned after success.

Unequipped NPCs select Unarmed; equipped NPCs infer attack tags from weapon inventory types. Map generation freezes head descendants outside portraits, always reduces keys conservatively, removes Mount/Sleep, deduplicates practically identical animations within each name family, and retains at most four variants per family. Attack Slam and other tagged families stay distinct. [Animation export](../rendering/animations.md) owns tolerances and optimization details.

## Icons

Each completed model enters a bounded icon queue while remaining models export. Up to three tabs share one installed headless browser. They use the portrait camera, frozen Stand pose and 1.7× framing, capturing at 384 pixels and producing 128-pixel ClassicHD2.0 BTN/DISBTN BLPs under `ReplaceableTextures/CommandButtons/` and `CommandButtonsDisabled/`. Unit `uico` points to the shared active icon.

`npcs/icons.json` keys model/texture contents and render settings. Fresh export rebuilds icons; destructible mode skips rendering and icon-cache changes. Browser reads drain before model deduplication or staged-cache cleanup deletes files. Viewer availability allows 30 seconds. Browser, camera, capture or encoding failures use default Footman icons and show yellow completion warnings without failing the map. Explicit cancellation still stops export.

## Voices

`resources/npc-voices.json` stores 34,201 exact NPC-name/base-model pairs from Wrath, with a display-ID index independent of exported names, skins, equipment and model deduplication. Names sharing a model can use different voices. The offline resource loads once; map export needs no LLM/API key.

Version 1 stores `sounds` (index 0 is silence), `models` with path/display/voice indexes, pair count and source-result checksum. Export writes unit `usnd` only. Trigger proxies (`flags_extra & 0x80`), intentional silence and unknown pairs get an empty field rather than inherited Footman sounds. Unknown pairs are logged. Voice choices are approximate and editable in World Editor.

## Death audio

Client `CreatureDisplayInfo` → `CreatureModelData` → `CreatureSoundData.SoundDeathID` → `SoundKitEntry` supplies death audio; AzerothCore supplies display IDs but not the complete lookup/audio. Display overrides precede model defaults, existing silent records remain silent, and trigger proxies are skipped. The first variant is the lowest SoundKitEntry record ID with a valid audio file.

Export deduplicates required files under `Sounds/WoW/Death/<fileDataID>.wav` and registers their imports. Ogg Vorbis, MP3 and PCM WAV become mono 16-bit WAV in-process. The persistent death trigger uses `wowDeathSounds[GetUnitTypeId(unit)]`, playing at unit X/Y and Z=0 with `play3dSound` defaults: volume 127, distances 0–7000, fade 10/10, DefaultEAXON and KillSoundWhenDone. Re-export replaces the lookup; disabling NPC unit export removes the trigger. Silent/unknown types do nothing, and WC3 sound sets remain assigned.

## Audit and verification

`units.json` records source identity/stats, owner, type code, exclusion reason and model/icon paths. Cached-model metadata scans decode animation/event/attachment chunks without loading geometry/textures into the conversion graph.

The bundled SQLite includes expanded creature/item templates, class-level stats and movement overrides. Regenerate it through `generate:acore-sqlite`. Older schemas use conservative level-based defaults; incomplete expanded bundles fail instead of silently losing NPCs.

Run `go -C golang test ./internal/converter/mapexporter/...` for field, faction, asset and startup coverage. Changes apply on the next map export; object-data edits need a fresh WC3 session. Native combat, audio, animation and pathing still require in-game verification.
