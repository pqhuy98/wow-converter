package metadata

import (
	"log"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

const colorIndexNone = 65535

// ExtractMDLGeosetAnim builds geoset color/alpha animations from M2 color tracks.
func (f *File) ExtractMDLGeosetAnim() {
	if !f.IsLoaded || !f.IsM2() || f.mdl == nil || len(f.colors) == 0 {
		return
	}

	geosetForSubmesh := func(submeshIndex int) *components.Geoset {
		if submeshIndex < 0 || submeshIndex >= len(f.skin.SubMeshes) {
			return nil
		}
		if !f.skin.SubMeshes[submeshIndex].Enabled {
			return nil
		}
		enabledIdx := 0
		for i, sm := range f.skin.SubMeshes {
			if !sm.Enabled {
				continue
			}
			if i == submeshIndex {
				if enabledIdx < len(f.mdl.Geosets) {
					return f.mdl.Geosets[enabledIdx]
				}
				return nil
			}
			enabledIdx++
		}
		return nil
	}

	var result []components.GeosetAnim
	for _, tu := range f.skin.TextureUnits {
		if tu.ColorIndex == colorIndexNone || tu.ColorIndex >= len(f.colors) {
			continue
		}
		geoset := geosetForSubmesh(tu.SkinSectionIndex)
		if geoset == nil {
			if tu.SkinSectionIndex >= 0 && tu.SkinSectionIndex < len(f.skin.SubMeshes) && f.skin.SubMeshes[tu.SkinSectionIndex].Enabled {
				log.Printf("%s %d %v", ansi.Red("geoset not found"), tu.SkinSectionIndex, f.skin.SubMeshes[tu.SkinSectionIndex])
			}
			continue
		}
		wowColor := f.colors[tu.ColorIndex]
		ga := components.GeosetAnim{
			Geoset: geoset,
		}
		if track, ok := parseM2TrackRaw(firstKey(wowColor, "color", "Color")); ok {
			transform := func(v []float64) imath.Vector3 {
				if len(v) < 3 {
					return imath.Vector3{}
				}
				return imath.Vector3{v[2], v[1], v[0]}
			}
			if trackRawIsStatic(track) && len(track.Values[0][0]) >= 3 {
				ga.Color = &components.AnimatedOrStatic[imath.Vector3]{Static: true, Value: transform(track.Values[0][0])}
			} else if anim := f.m2TrackToAnimation(track, components.AnimTypeColor, func(v []float64) any {
				return transform(v)
			}); anim != nil {
				ga.Color = &components.AnimatedOrStatic[imath.Vector3]{Static: false, Anim: anim}
			}
		}
		if track, ok := parseM2TrackRaw(firstKey(wowColor, "alpha", "Alpha")); ok {
			transform := func(v []float64) float64 {
				if len(v) == 0 {
					return float64(1)
				}
				return v[0] / 32767
			}
			if trackRawIsStatic(track) && len(track.Values[0][0]) > 0 {
				ga.Alpha = &components.AnimatedOrStatic[float64]{Static: true, Value: transform(track.Values[0][0])}
			} else if anim := f.m2TrackToAnimation(track, components.AnimTypeAlpha, func(v []float64) any {
				return transform(v)
			}); anim != nil {
				ga.Alpha = &components.AnimatedOrStatic[float64]{Static: false, Anim: anim}
			}
		}
		if ga.Color != nil || ga.Alpha != nil {
			result = append(result, ga)
		}
	}
	f.mdl.GeosetAnims = result
}

func trackRawIsStatic(track m2TrackRaw) bool {
	return len(track.Values) == 1 && len(track.Values[0]) == 1
}
