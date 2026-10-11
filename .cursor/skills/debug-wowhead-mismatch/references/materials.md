# Materials and textures

Inspect MDL `Textures` and `Materials`: each geoset's bitmap, `FilterMode`, and `Unlit`. Present geometry with a wrong surface points to textures/materials; absent geometry points to the geoset mask.

`Chosen skin` in the export log is a texture basename, not a `.skin` file. The score counts texture hits when no geoset contributed; geoset contributions are multiplied by 1000000. A small score means geosets did not select the skin. Confidence below 100% means at least one requested texture id was absent.

Creature color variants use that skin selection. Dressing-room dye uses the item-bonus display modifier (`displayModByBonus` in `golang/internal/wowhead/gatherer_items.go`).

For WMO black/magenta, shader 23 binds diffuse from texture2, skipping a filled texture1, in `wmoDiffuseFileID`. Missing shader-23 environment textures must not emit an empty pass (ADR 0002). M2 native opaque/alpha coverage lives in `golang/internal/converter/wowmodel/direct/m2/texture_native_opaque.go`.
