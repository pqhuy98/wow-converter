# Ground foliage: failed experiment

The WC3 ground-foliage port was removed on 2026-10-23 after user review. From WC3's top-down camera, the plants were tiny and barely noticeable, while their retained animations added rendering cost. The visual benefit did not justify the cost. This is the user's observed result; no controlled FPS benchmark or isolated animation-cost measurement was recorded.

## What was tried

The experiment reconstructed grass and small plants from terrain texture-layer effect IDs, `GroundEffectTexture` weighted choices/density, and `GroundEffectDoodad` models. MCAL coverage, MCNK suppression masks, terrain holes and submerged cells restricted placement. Plants followed the source terrain surface. The distribution was deterministic but approximated WoW's proprietary placement algorithm.

The first version grouped plants into spatial patches. Each patch produced a separate doodad type, cluttering the World Editor object list, so grouping was rejected. The replacement placed each plant individually and shared one doodad type per source model.

The top-down defaults were 25% density and a 128-pixel texture cap. Full reconstructed density and source texture resolution were also configurable. Source geometry and animations were retained, and foliage contributed no navigation collision or pathing. Smaller textures did not remove the animation work or the cost of many individual placements.

Offline Elwynn validation produced 1,535 individual plants sharing 11 doodad types at the default settings, versus 6,242 plants at full density. Texture output was checked at 128 pixels versus 256-pixel source textures. These checks established export and serialization behavior; they did not establish acceptable in-game visibility or performance.

![Historical foliage export controls, removed after the experiment](foliage-settings.png)

## Outcome

The WC3 foliage placement code, dedicated asset export, settings, generation API fields, UI controls and experiment-specific tests were removed. The `foliage` generation option and `foliage.json` manifest described by the experiment are no longer supported. Regenerate existing maps to remove previously exported foliage placements.

Explicitly placed trees and bushes continue through the ordinary scenery export path. The pre-existing WoW foliage reference/metadata exporter remains separate from WC3 procedural placement.

Independent fixes discovered during the experiment were retained: DB2 inline-field offsets after packed fields, narrow integer decoding, deterministic object-path ordering, and retrying foliage-table loads after cancellation instead of permanently poisoning the shared cache.

Any future attempt needs a convincing top-down visual benefit and measured WC3 rendering cost before restoring automatic plant placement. Reduced texture resolution alone did not make this approach worthwhile.
