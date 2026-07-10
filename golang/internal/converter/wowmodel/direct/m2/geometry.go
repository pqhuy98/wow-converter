package directm2

import (
	objpkg "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// ObjMesh is a mesh section for OBJ-equivalent output.
type ObjMesh struct {
	Name      string
	Triangles []int
	MatName   string
}

// BuildMeshes mirrors M2Exporter.exportAsOBJ submesh loop.
func BuildMeshes(loader *m2.Loader, skin *m2.Skin, geosetMask []m2export.GeosetMaskEntry, validTextures map[any]m2export.TextureManifestEntry, dataTextures map[int]struct{}) []ObjMesh {
	var meshes []ObjMesh
	for mI, mesh := range skin.SubMeshes {
		if geosetMask != nil && mI < len(geosetMask) && !geosetMask[mI].Checked {
			continue
		}
		verts := make([]int, mesh.TriangleCount)
		for vI := uint16(0); vI < mesh.TriangleCount; vI++ {
			idx := skin.Triangles[int(mesh.TriangleStart)+int(vI)]
			vert := int(skin.Indices[idx])
			verts[vI] = vert
		}

		var matName string
		for _, texUnit := range skin.TextureUnits {
			if texUnit.SkinSectionIndex == uint16(mI) {
				comboIdx := int(texUnit.TextureComboIndex)
				if comboIdx < len(loader.TextureCombos) {
					texIdx := int(loader.TextureCombos[comboIdx])
					if texIdx < len(loader.Textures) {
						tex := loader.Textures[texIdx]
						texType := loader.TextureTypes[texIdx]
						fileDataID := any(tex.FileDataID)
						if tex.FileDataID == 0 && tex.FileName != "" {
							fileDataID = tex.FileName
						}
						if dataTextures != nil {
							if _, ok := dataTextures[int(texType)]; ok {
								fileDataID = "data-" + itoa(int(texType))
							}
						}
						if entry, ok := validTextures[fileDataID]; ok {
							matName = entry.MatName
						}
					}
				}
				break
			}
		}
		meshes = append(meshes, ObjMesh{
			Name: m2export.GetGeosetName(mI, int(mesh.SubmeshID)), Triangles: verts, MatName: matName,
		})
	}
	return meshes
}

// BuildObjResult builds OBJ-equivalent structure for the visible model.
func BuildObjResult(loader *m2.Loader, meshes []ObjMesh, modelName, mtlLib string) objpkg.Result {
	uvLayers := [][]float32{loader.UV}
	if len(loader.UV2) > 0 {
		uvLayers = append(uvLayers, loader.UV2)
	}
	return buildRawObjResult(toFloat64Slice(loader.Vertices), toFloat64Slice(loader.Normals), uvLayers, meshes, modelName, mtlLib)
}

// BuildRawObjResult builds an OBJ-equivalent result from raw geometry arrays.
func BuildRawObjResult(verts, normals []float64, uvLayers [][]float32, meshes []ObjMesh, modelName, mtlLib string) objpkg.Result {
	return buildRawObjResult(verts, normals, uvLayers, meshes, modelName, mtlLib)
}

// BuildCollisionObjResult builds collision mesh OBJ structure.
func BuildCollisionObjResult(loader *m2.Loader, modelName string) objpkg.Result {
	return buildRawObjResult(toFloat64Slice(loader.CollisionPositions), toFloat64Slice(loader.CollisionNormals), nil,
		[]ObjMesh{{Name: "Collision", Triangles: intsFromU16(loader.CollisionIndices)}}, modelName, "")
}

func buildRawObjResult(verts, normals []float64, uvLayers [][]float32, meshes []ObjMesh, modelName, mtlLib string) objpkg.Result {
	used := map[int]struct{}{}
	for _, mesh := range meshes {
		for _, idx := range mesh.Triangles {
			used[idx] = struct{}{}
		}
	}

	vertMap := map[int]int{}
	normalMap := map[int]int{}
	uvMap := map[int]int{}
	var vertices []objpkg.Vertex
	for i, j, u := 0, 0, 0; i+2 < len(verts); i, j = i+3, j+1 {
		if _, ok := used[j]; !ok {
			continue
		}
		vertMap[j] = u
		u++
		vertices = append(vertices, objpkg.Vertex{X: verts[i], Y: verts[i+1], Z: verts[i+2]})
	}
	var vertexNormals []objpkg.Vertex
	for i, j, u := 0, 0, 0; i+2 < len(normals); i, j = i+3, j+1 {
		if _, ok := used[j]; !ok {
			continue
		}
		normalMap[j] = u
		u++
		vertexNormals = append(vertexNormals, objpkg.Vertex{X: normals[i], Y: normals[i+1], Z: normals[i+2]})
	}
	hasUV := len(uvLayers) > 0
	var textureCoords, textureCoords2 []objpkg.TextureVertex
	for uvIndex, uv := range uvLayers {
		target := &textureCoords
		if uvIndex == 1 {
			target = &textureCoords2
		}
		for i, j, u := 0, 0, 0; i+1 < len(uv); i, j = i+2, j+1 {
			if _, ok := used[j]; !ok {
				continue
			}
			if uvIndex == 0 {
				uvMap[j] = u
			}
			u++
			*target = append(*target, objpkg.TextureVertex{U: float64(uv[i]), V: float64(uv[i+1]), W: 0})
		}
	}

	var faces []objpkg.Face
	currentMaterial := ""
	for groupID, mesh := range meshes {
		group := objpkg.Group{Name: mesh.Name, ID: groupID + 1}
		if mesh.MatName != "" {
			currentMaterial = mesh.MatName
		}
		for i := 0; i+2 < len(mesh.Triangles); i += 3 {
			face := objpkg.Face{Group: group, Material: currentMaterial, SmoothingGroup: 1}
			for k := 0; k < 3; k++ {
				t := mesh.Triangles[i+k]
				fv := objpkg.FaceVertex{
					VertexIndex:       vertMap[t] + 1,
					VertexNormalIndex: normalMap[t] + 1,
				}
				if hasUV {
					fv.TextureCoordsIndex = uvMap[t] + 1
				}
				face.Vertices[k] = fv
			}
			faces = append(faces, face)
		}
	}

	model := objpkg.Model{
		Name: modelName, Vertices: vertices, VertexNormals: vertexNormals,
		TextureCoords: textureCoords, TextureCoords2: textureCoords2, Faces: faces,
	}
	var mtlLibs []string
	if mtlLib != "" {
		mtlLibs = []string{mtlLib}
	}
	return objpkg.Result{Models: []objpkg.Model{model}, MaterialLibraries: mtlLibs}
}

func toFloat64Slice(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

func intsFromU16(in []uint16) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
