package character

import "github.com/pqhuy98/wow-converter/internal/wow/character/meta"

// The dependency-free schema lives beside this package so CASC contracts can
// share it without importing the character resolver and its runtime caches.
type CharComponentTextureSection = meta.CharComponentTextureSection
type ChrModelMaterialRow = meta.ChrModelMaterialRow
type ChrModelTextureLayerRow = meta.ChrModelTextureLayerRow
type ChrCustomizationMaterialEntry = meta.ChrCustomizationMaterialEntry
type CharMetaChoiceMaterial = meta.CharMetaChoiceMaterial
type ChoiceMeta = meta.ChoiceMeta
