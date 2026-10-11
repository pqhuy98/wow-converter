# Missing geometry

Export `format: "mdl"` and `formatVersion: "1000"`; inspect geoset `Name` lines and vertex bounds. Do not hex-edit MDX or download M2 merely to parse its submeshes.

`GetGeosetName` in `golang/internal/wow/export/m2/geoset_mapper.go` labels submesh id 0 as `Geoset` plus its index. Other ids use the group label plus `id % 100`: id 2 is `Hair2`, id 702 is `Ears2`.

The mask is `BuildGeosetMaskForSkin` in `golang/internal/converter/wowmodel/direct/m2/convert.go`. With no extra geosets it keeps ids ending in `0` or `01`, every id under 100, and a 100-group with one distinct non-default id. Two non-default variants in one group stay off. With extra geosets, ids 1–899 start off and only listed ids turn on; id 0 and ids at or above 900 stay on.

Wowhead `meta/npc/<displayId>.json` can have a `Creature` object with null `CreatureGeosetData`: treat it as an empty list. `mergeNpcMeta` keeps a present Wowhead creature, so that empty list does not fall through to database geosets.

The MDL contains only kept geosets. To inspect a dropped id, temporarily log the mask for that export and remove the log afterward. A default-mask change affects every creature using it.
