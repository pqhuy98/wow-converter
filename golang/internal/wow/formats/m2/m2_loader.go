package m2

import (
	"context"
	"fmt"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
)

const (
	chunkSFID = 0x44494653
	chunkTXID = 0x44495854
	chunkSKID = 0x44494B53
	chunkBFID = 0x44494642
	chunkAFID = 0x44494641
)

// AnimFileIDEntry maps animation IDs to file data IDs.
type AnimFileIDEntry struct {
	AnimID, SubAnimID uint16
	FileDataID        uint32
}

// Loader parses M2 model files.
type Loader struct {
	Data *buffer.Buffer

	IsLoaded       bool
	Version        uint32
	Name           string
	Flags          uint32
	ViewCount      uint32
	MD21Ofs        int
	SkeletonFileID uint32
	BoneFileIDs    []uint32
	Skins          []*Skin
	LodSkins       []*Skin
	AnimFileIDs    []AnimFileIDEntry
	AnimFiles      map[int]*buffer.Buffer
	isAnimLoaded   bool

	Vertices                []float32
	Normals                 []float32
	UV                      []float32
	UV2                     []float32
	BoneWeights             []uint8
	BoneIndices             []uint8
	Textures                []TextureEntry
	TextureTypes            []uint32
	TextureCombos           []uint16
	Materials               []MaterialEntry
	Animations              []AnimationEntry
	Bones                   []BoneEntry
	Attachments             []AttachmentEntry
	TextureTransforms       []TextureTransformEntry
	TextureTransformsLookup []uint16
	TransparencyLookup      []uint16
	BoundingBox             CAABox
	BoundingSphereRadius    float32
	CollisionBox            CAABox
	CollisionSphereRadius   float32
	CollisionPositions      []float32
	CollisionNormals        []float32
	CollisionIndices        []uint16
	Lights                  []LightEntry
	Cameras                 []CameraEntry
	CameraLookup            []uint16
	RibbonEmitters          []RibbonEmitterEntry
	ParticleEmitters        []ParticleEmitterEntry
	Colors                  []ColorEntry
	TextureWeights          []any

	getFile func(ctx context.Context, fileDataID uint32) ([]byte, error)
}

// NewLoader creates an M2 loader.
func NewLoader(data *buffer.Buffer, getFile func(context.Context, uint32) ([]byte, error)) *Loader {
	return &Loader{Data: data, getFile: getFile, AnimFiles: map[int]*buffer.Buffer{}}
}

// GetFile resolves a CASC file by data ID.
func (l *Loader) GetFile(ctx context.Context, fileDataID uint32) ([]byte, error) {
	return l.getFile(ctx, fileDataID)
}

// Load parses the M2 file.
func (l *Loader) Load(ctx context.Context) error {
	if l.IsLoaded {
		return nil
	}
	for l.Data.RemainingBytes() > 0 {
		chunkID := readU32(l.Data)
		chunkSize := int(readU32(l.Data))
		nextChunkPos := l.Data.Offset() + chunkSize
		switch chunkID {
		case constants.MagicMD21:
			if err := l.parseChunkMD21(); err != nil {
				return err
			}
		case chunkSFID:
			l.parseChunkSFID(chunkSize)
		case chunkTXID:
			l.parseChunkTXID()
		case chunkSKID:
			l.SkeletonFileID = readU32(l.Data)
		case chunkBFID:
			l.BoneFileIDs = readU32Slice(l.Data, chunkSize/4)
		case chunkAFID:
			l.parseChunkAFID(chunkSize)
		}
		l.Data.Seek(nextChunkPos)
	}
	for _, skin := range l.Skins {
		skin.getFile = l.getFile
	}
	for _, skin := range l.LodSkins {
		skin.getFile = l.getFile
	}
	l.IsLoaded = true
	_ = ctx
	return nil
}

// GetSkin loads and returns skin geometry by index.
func (l *Loader) GetSkin(ctx context.Context, index int) (*Skin, error) {
	if index < 0 || index >= len(l.Skins) {
		return nil, fmt.Errorf("skin index %d out of range", index)
	}
	skin := l.Skins[index]
	if err := skin.Load(ctx); err != nil {
		return nil, err
	}
	return skin, nil
}

func (l *Loader) parseChunkTXID() {
	if l.Textures == nil {
		panic("cannot parse TXID chunk in M2 before MD21 chunk")
	}
	for i := range l.Textures {
		l.Textures[i].FileDataID = readU32(l.Data)
	}
}

func (l *Loader) parseChunkSFID(chunkSize int) {
	if l.ViewCount == 0 {
		panic("cannot parse SFID chunk in M2 before MD21 chunk")
	}
	lodSkinCount := (chunkSize / 4) - int(l.ViewCount)
	l.Skins = make([]*Skin, l.ViewCount)
	l.LodSkins = make([]*Skin, lodSkinCount)
	for i := 0; i < int(l.ViewCount); i++ {
		l.Skins[i] = NewSkin(readU32(l.Data))
	}
	for i := 0; i < lodSkinCount; i++ {
		l.LodSkins[i] = NewSkin(readU32(l.Data))
	}
}

func (l *Loader) parseChunkAFID(chunkSize int) {
	entryCount := chunkSize / 8
	entries := make([]AnimFileIDEntry, entryCount)
	for i := 0; i < entryCount; i++ {
		entries[i] = AnimFileIDEntry{
			AnimID: readU16(l.Data), SubAnimID: readU16(l.Data),
			FileDataID: readU32(l.Data),
		}
	}
	l.AnimFileIDs = entries
}

func (l *Loader) parseChunkMD21() error {
	ofs := l.Data.Offset()
	magic := readU32(l.Data)
	if magic != constants.MagicMD20 {
		return fmt.Errorf("invalid M2 magic: %08x", magic)
	}
	l.Version = readU32(l.Data)
	l.MD21Ofs = ofs
	l.parseMD21ModelName(ofs)
	l.Flags = readU32(l.Data)
	l.Data.Move(8) // globalLoops
	l.parseMD21Animations(ofs)
	l.Data.Move(8) // animationLookup
	l.parseMD21Bones(ofs, false)
	l.Data.Move(8)
	l.parseMD21Vertices(ofs)
	l.ViewCount = readU32(l.Data)
	l.parseMD21Colors(ofs)
	l.parseMD21Textures(ofs)
	l.parseMD21TextureWeights(ofs)
	l.parseMD21TextureTransforms(ofs)
	l.Data.Move(8) // replaceableTextureLookup
	l.parseMD21Materials(ofs)
	l.Data.Move(8) // boneCombos
	l.parseMD21TextureCombos(ofs)
	l.Data.Move(8) // textureTransformBoneMap
	l.parseMD21TransparencyLookup(ofs)
	l.parseMD21TextureTransformLookup(ofs)
	l.parseMD21Collision(ofs)
	l.parseMD21Attachments(ofs)
	l.Data.Move(16) // attachment indices + events
	l.parseMD21Lights(ofs)
	l.parseMD21Cameras(ofs)
	l.parseMD21CameraLookup(ofs)
	l.parseMD21RibbonEmitters(ofs)
	l.parseMD21ParticleEmitters(ofs)
	return nil
}

func (l *Loader) parseMD21ModelName(ofs int) {
	nameLen := int(readU32(l.Data))
	nameOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	if nameLen > 1 {
		l.Data.Seek(nameOfs + ofs)
		l.Name = l.Data.ReadString(nameLen-1, "utf8")
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Animations(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	if count == 0 {
		return
	}
	l.Data.Seek(tableOfs + ofs)
	l.Animations = make([]AnimationEntry, count)
	for i := 0; i < count; i++ {
		entry := AnimationEntry{
			ID: readU16(l.Data), VariationIndex: readU16(l.Data),
			Duration:  readU32(l.Data),
			MoveSpeed: l.Data.ReadFloatLE().(float32),
			Flags:     readU32(l.Data),
		}
		entry.Frequency = uint32(int16(readU16(l.Data)))
		_ = readU16(l.Data)
		entry.ReplayMin = readU32(l.Data)
		entry.ReplayMax = readU32(l.Data)
		entry.BlendTimeIn = readU16(l.Data)
		entry.BlendTimeOut = readU16(l.Data)
		entry.BoxPosMin = readFloat3(l.Data)
		entry.BoxPosMax = readFloat3(l.Data)
		entry.BoxRadius = l.Data.ReadFloatLE().(float32)
		entry.VariationNext = int16(readU16(l.Data))
		entry.AliasNext = int16(readU16(l.Data))
		l.Animations[i] = entry
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Bones(ofs int, useAnims bool) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	if count == 0 {
		return
	}
	l.Data.Seek(tableOfs + ofs)
	animFiles := l.animFilesFor(useAnims)
	l.Bones = make([]BoneEntry, count)
	for i := 0; i < count; i++ {
		bone := BoneEntry{
			BoneID: int32(readU32(l.Data)), Flags: readU32(l.Data),
			ParentBone: int16(readU16(l.Data)), SubMeshID: readU16(l.Data),
			BoneNameCRC: readU32(l.Data),
			Translation: ReadM2Track(l.Data, ofs, TrackFloat3, animFiles),
			Rotation:    ReadM2Track(l.Data, ofs, TrackCompQuat, animFiles),
			Scale:       ReadM2Track(l.Data, ofs, TrackFloat3, animFiles),
			Pivot:       readFloat3(l.Data),
		}
		convertBoneCoords(&bone)
		l.Bones[i] = bone
	}
	l.Data.Seek(base)
}

func (l *Loader) animFilesFor(useAnims bool) map[int]*buffer.Buffer {
	if useAnims {
		return l.AnimFiles
	}
	return nil
}

// LoadAnims loads external .anim files and re-parses bone tracks.
func (l *Loader) LoadAnims(ctx context.Context) error {
	if l.isAnimLoaded {
		return nil
	}
	for i := range l.Animations {
		if err := l.LoadAnimsForIndex(ctx, i, false); err != nil {
			return err
		}
	}
	l.Data.Seek(l.MD21Ofs + 44)
	l.parseMD21Bones(l.MD21Ofs, true)
	l.isAnimLoaded = true
	return nil
}

// LoadAnimsForIndex loads a single animation's .anim file.
func (l *Loader) LoadAnimsForIndex(ctx context.Context, animationIndex int, reparseBones bool) error {
	if _, ok := l.AnimFiles[animationIndex]; ok {
		return nil
	}
	if animationIndex >= len(l.Animations) {
		return nil
	}
	animation := l.Animations[animationIndex]
	if animation.Flags&0x40 == 0x40 {
		for animation.Flags&0x40 == 0x40 && int(animation.AliasNext) < len(l.Animations) {
			animation = l.Animations[animation.AliasNext]
		}
	}
	if animation.Flags&0x20 == 0x20 {
		return nil
	}
	for _, entry := range l.AnimFileIDs {
		if entry.AnimID != animation.ID || entry.SubAnimID != animation.VariationIndex {
			continue
		}
		if entry.FileDataID == 0 {
			return nil
		}
		raw, err := l.getFile(ctx, entry.FileDataID)
		if err != nil {
			return err
		}
		animIsChunked := l.Flags&0x200000 == 0x200000 || l.SkeletonFileID > 0
		animLoader := NewAnimLoader(buffer.From(raw))
		animLoader.Load(animIsChunked)
		if animLoader.SkeletonBoneData != nil {
			l.AnimFiles[animationIndex] = buffer.From(animLoader.SkeletonBoneData)
		} else if animLoader.AnimData != nil {
			l.AnimFiles[animationIndex] = buffer.From(animLoader.AnimData)
		}
		if reparseBones {
			l.Data.Seek(l.MD21Ofs + 44)
			l.parseMD21Bones(l.MD21Ofs, true)
		}
		return nil
	}
	return nil
}

func convertBoneCoords(bone *BoneEntry) {
	for a := range bone.Translation.Values {
		for j := range bone.Translation.Values[a] {
			v := bone.Translation.Values[a][j]
			if len(v) >= 3 {
				dx, dy, dz := v[0], v[1], v[2]
				bone.Translation.Values[a][j] = []float64{dx, dz, -dy}
			}
		}
	}
	for a := range bone.Rotation.Values {
		for j := range bone.Rotation.Values[a] {
			v := bone.Rotation.Values[a][j]
			if len(v) >= 4 {
				dx, dy, dz, dw := v[0], v[1], v[2], v[3]
				bone.Rotation.Values[a][j] = []float64{dx, dz, -dy, dw}
			}
		}
	}
	for a := range bone.Scale.Values {
		for j := range bone.Scale.Values[a] {
			v := bone.Scale.Values[a][j]
			if len(v) >= 3 {
				dx, dy, dz := v[0], v[1], v[2]
				bone.Scale.Values[a][j] = []float64{dx, dz, dy}
			}
		}
	}
	p := bone.Pivot
	bone.Pivot = [3]float32{p[0], p[2], -p[1]}
}

func (l *Loader) parseMD21Vertices(ofs int) {
	count := int(readU32(l.Data))
	vertOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	l.Data.Seek(vertOfs + ofs)
	l.Vertices = make([]float32, count*3)
	l.Normals = make([]float32, count*3)
	l.UV = make([]float32, count*2)
	l.UV2 = make([]float32, count*2)
	l.BoneWeights = make([]uint8, count*4)
	l.BoneIndices = make([]uint8, count*4)
	for i := 0; i < count; i++ {
		l.Vertices[i*3] = l.Data.ReadFloatLE().(float32)
		l.Vertices[i*3+2] = -l.Data.ReadFloatLE().(float32)
		l.Vertices[i*3+1] = l.Data.ReadFloatLE().(float32)
		for x := 0; x < 4; x++ {
			l.BoneWeights[i*4+x] = uint8(l.Data.ReadUInt8().(int64))
		}
		for x := 0; x < 4; x++ {
			l.BoneIndices[i*4+x] = uint8(l.Data.ReadUInt8().(int64))
		}
		l.Normals[i*3] = l.Data.ReadFloatLE().(float32)
		l.Normals[i*3+2] = -l.Data.ReadFloatLE().(float32)
		l.Normals[i*3+1] = l.Data.ReadFloatLE().(float32)
		l.UV[i*2] = l.Data.ReadFloatLE().(float32)
		l.UV[i*2+1] = -(l.Data.ReadFloatLE().(float32) - 1)
		l.UV2[i*2] = l.Data.ReadFloatLE().(float32)
		l.UV2[i*2+1] = -(l.Data.ReadFloatLE().(float32) - 1)
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Colors(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	if count == 0 {
		return
	}
	l.Data.Seek(tableOfs + ofs)
	l.Colors = make([]ColorEntry, count)
	for i := 0; i < count; i++ {
		l.Colors[i] = ColorEntry{
			Color: ReadM2Track(l.Data, ofs, TrackFloat3, nil),
			Alpha: ReadM2Track(l.Data, ofs, TrackInt16, nil),
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Textures(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	l.Textures = make([]TextureEntry, count)
	l.TextureTypes = make([]uint32, count)
	if count > 0 {
		l.Data.Seek(tableOfs + ofs)
		for i := 0; i < count; i++ {
			texType := readU32(l.Data)
			flags := readU32(l.Data)
			nameLen := int(readU32(l.Data))
			nameOfs := int(readU32(l.Data))
			l.TextureTypes[i] = texType
			l.Textures[i] = TextureEntry{Flags: flags}
			if texType == 0 && nameOfs > 0 {
				pos := l.Data.Offset()
				l.Data.Seek(nameOfs)
				if nameLen > 0 {
					fileName := strings.TrimRight(l.Data.ReadString(nameLen, "utf8"), "\x00")
					fileName = strings.ReplaceAll(fileName, "\\", "/")
					l.Textures[i].FileName = fileName
					if id, ok := archivecasc.GetByFilename(fileName); ok && id > 0 {
						l.Textures[i].FileDataID = uint32(id)
					}
				}
				l.Data.Seek(pos)
			}
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21TextureWeights(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	_ = count
	_ = tableOfs
	_ = ofs
}

func (l *Loader) parseMD21TextureTransforms(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	l.TextureTransforms = make([]TextureTransformEntry, count)
	if count > 0 {
		l.Data.Seek(tableOfs + ofs)
		for i := 0; i < count; i++ {
			l.TextureTransforms[i] = TextureTransformEntry{
				Translation: ReadM2Track(l.Data, ofs, TrackFloat3, nil),
				Rotation:    ReadM2Track(l.Data, ofs, TrackFloat4, nil),
				Scaling:     ReadM2Track(l.Data, ofs, TrackFloat3, nil),
			}
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Materials(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	l.Materials = make([]MaterialEntry, count)
	if count > 0 {
		l.Data.Seek(tableOfs + ofs)
		for i := 0; i < count; i++ {
			l.Materials[i] = MaterialEntry{
				Flags: readU16(l.Data), BlendingMode: readU16(l.Data),
			}
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21TextureCombos(ofs int) {
	l.TextureCombos = readM2ArrayU16(l.Data, ofs)
}

func (l *Loader) parseMD21TransparencyLookup(ofs int) {
	l.TransparencyLookup = readM2ArrayU16(l.Data, ofs)
}

func (l *Loader) parseMD21TextureTransformLookup(ofs int) {
	l.TextureTransformsLookup = readM2ArrayU16(l.Data, ofs)
}

func (l *Loader) parseMD21Collision(ofs int) {
	l.BoundingBox = readCAABBox(l.Data)
	l.BoundingSphereRadius = l.Data.ReadFloatLE().(float32)
	l.CollisionBox = readCAABBox(l.Data)
	l.CollisionSphereRadius = l.Data.ReadFloatLE().(float32)

	indicesCount := int(readU32(l.Data))
	indicesOfs := int(readU32(l.Data))
	positionsCount := int(readU32(l.Data))
	positionsOfs := int(readU32(l.Data))
	normalsCount := int(readU32(l.Data))
	normalsOfs := int(readU32(l.Data))

	base := l.Data.Offset()

	if indicesCount > 0 {
		l.Data.Seek(indicesOfs + ofs)
		l.CollisionIndices = readU16Slice(l.Data, indicesCount)
	}

	if positionsCount > 0 {
		l.Data.Seek(positionsOfs + ofs)
		l.CollisionPositions = make([]float32, positionsCount*3)
		for i := 0; i < positionsCount; i++ {
			l.CollisionPositions[i*3] = l.Data.ReadFloatLE().(float32)
			l.CollisionPositions[i*3+2] = -l.Data.ReadFloatLE().(float32)
			l.CollisionPositions[i*3+1] = l.Data.ReadFloatLE().(float32)
		}
	}

	if normalsCount > 0 {
		l.Data.Seek(normalsOfs + ofs)
		l.CollisionNormals = make([]float32, normalsCount*3)
		for i := 0; i < normalsCount; i++ {
			l.CollisionNormals[i*3] = l.Data.ReadFloatLE().(float32)
			l.CollisionNormals[i*3+2] = -l.Data.ReadFloatLE().(float32)
			l.CollisionNormals[i*3+1] = l.Data.ReadFloatLE().(float32)
		}
	}

	l.Data.Seek(base)
}

func (l *Loader) parseMD21Attachments(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	base := l.Data.Offset()
	if count > 0 && tableOfs > 0 {
		l.Attachments = make([]AttachmentEntry, count)
		l.Data.Seek(tableOfs + ofs)
		for i := 0; i < count; i++ {
			entry := AttachmentEntry{
				ID: readU32(l.Data), Bone: readU16(l.Data), Unknown: readU16(l.Data),
			}
			pos := readFloat3(l.Data)
			entry.Position = [3]float32{pos[0], pos[2], -pos[1]}
			_ = ReadM2Track(l.Data, ofs, TrackUint8, nil)
			l.Attachments[i] = entry
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Lights(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	if count == 0 || tableOfs == 0 {
		return
	}
	base := l.Data.Offset()
	l.Data.Seek(tableOfs + ofs)
	l.Lights = make([]LightEntry, count)
	for i := 0; i < count; i++ {
		typ := readU16(l.Data)
		bone := int16(readU16(l.Data))
		pos := readFloat3(l.Data)
		entry := LightEntry{
			Type:             typ,
			Bone:             bone,
			Position:         [3]float32{pos[0], pos[2], -pos[1]},
			AmbientColor:     ReadM2Track(l.Data, ofs, TrackFloat3, nil),
			AmbientIntensity: ReadM2Track(l.Data, ofs, TrackFloat, nil),
			DiffuseColor:     ReadM2Track(l.Data, ofs, TrackFloat3, nil),
			DiffuseIntensity: ReadM2Track(l.Data, ofs, TrackFloat, nil),
			AttenuationStart: ReadM2Track(l.Data, ofs, TrackFloat, nil),
			AttenuationEnd:   ReadM2Track(l.Data, ofs, TrackFloat, nil),
			Visibility:       ReadM2Track(l.Data, ofs, TrackUint8, nil),
		}
		l.Lights[i] = entry
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21Cameras(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	if count == 0 || tableOfs == 0 {
		return
	}
	base := l.Data.Offset()
	l.Data.Seek(tableOfs + ofs)
	l.Cameras = make([]CameraEntry, count)
	for i := 0; i < count; i++ {
		typ := int32(readU32(l.Data))
		farClip := l.Data.ReadFloatLE().(float32)
		nearClip := l.Data.ReadFloatLE().(float32)
		_ = ReadM2SplineTrack(l.Data, ofs, TrackFloat3)
		posBase := readFloat3(l.Data)
		_ = ReadM2SplineTrack(l.Data, ofs, TrackFloat3)
		tgtBase := readFloat3(l.Data)
		_ = ReadM2SplineTrack(l.Data, ofs, TrackFloat)
		fov := Track{}
		saved := l.Data.Offset()
		if l.Data.RemainingBytes() >= 8 {
			fov = ReadM2SplineTrack(l.Data, ofs, TrackFloat)
		}
		if len(fov.Values) == 0 && len(fov.Timestamps) == 0 {
			l.Data.Seek(saved)
		}
		l.Cameras[i] = CameraEntry{
			Type:               typ,
			FarClip:            farClip,
			NearClip:           nearClip,
			PositionBase:       [3]float32{posBase[0], posBase[2], -posBase[1]},
			TargetPositionBase: [3]float32{tgtBase[0], tgtBase[2], -tgtBase[1]},
			FoV:                fov,
		}
	}
	l.Data.Seek(base)
}

func (l *Loader) parseMD21CameraLookup(ofs int) {
	l.CameraLookup = readM2ArrayU16(l.Data, ofs)
}

func (l *Loader) parseMD21RibbonEmitters(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	if count == 0 || tableOfs == 0 {
		return
	}
	base := l.Data.Offset()
	l.Data.Seek(tableOfs + ofs)
	l.RibbonEmitters = make([]RibbonEmitterEntry, count)
	for i := 0; i < count; i++ {
		ribbonID := readU32(l.Data)
		boneIndex := readU32(l.Data)
		pos := readFloat3(l.Data)
		l.RibbonEmitters[i] = RibbonEmitterEntry{
			RibbonID:                    ribbonID,
			BoneIndex:                   boneIndex,
			Position:                    [3]float32{pos[0], pos[1], pos[2]},
			TextureIndices:              readM2ArrayU16(l.Data, l.MD21Ofs),
			MaterialIndices:             readM2ArrayU16(l.Data, l.MD21Ofs),
			ColorTrack:                  ReadM2Track(l.Data, l.MD21Ofs, TrackFloat3, nil),
			AlphaTrack:                  ReadM2Track(l.Data, l.MD21Ofs, TrackInt16, nil),
			HeightAboveTrack:            ReadM2Track(l.Data, l.MD21Ofs, TrackFloat, nil),
			HeightBelowTrack:            ReadM2Track(l.Data, l.MD21Ofs, TrackFloat, nil),
			EdgesPerSecond:              l.Data.ReadFloatLE().(float32),
			EdgeLifetime:                l.Data.ReadFloatLE().(float32),
			Gravity:                     l.Data.ReadFloatLE().(float32),
			TextureRows:                 readU16(l.Data),
			TextureCols:                 readU16(l.Data),
			TexSlotTrack:                ReadM2Track(l.Data, l.MD21Ofs, TrackUint16, nil),
			VisibilityTrack:             ReadM2Track(l.Data, l.MD21Ofs, TrackUint8, nil),
			PriorityPlane:               int16(l.Data.ReadInt16LE().(int64)),
			RibbonColorIndex:            int8(l.Data.ReadInt8().(int64)),
			TextureTransformLookupIndex: int8(l.Data.ReadInt8().(int64)),
		}
	}
	l.Data.Seek(base)
}

func readFp69Vec2(data *buffer.Buffer) [2]float32 {
	x := readU16(data)
	y := readU16(data)
	return [2]float32{float32(x) / 512, float32(y) / 512}
}

func (l *Loader) parseMD21ParticleEmitters(ofs int) {
	count := int(readU32(l.Data))
	tableOfs := int(readU32(l.Data))
	if count == 0 || tableOfs == 0 {
		return
	}
	base := l.Data.Offset()
	l.Data.Seek(tableOfs + ofs)
	l.ParticleEmitters = make([]ParticleEmitterEntry, count)
	for i := 0; i < count; i++ {
		entry := ParticleEmitterEntry{
			ParticleID: readU32(l.Data),
			Flags:      readU32(l.Data),
		}
		pos := readFloat3(l.Data)
		entry.Position = [3]float32{pos[0], pos[2], -pos[1]}
		entry.Bone = readU16(l.Data)
		entry.TexturePacked = readU16(l.Data)
		_ = readU32(l.Data) // geometryModelNameLen
		_ = readU32(l.Data) // geometryModelNameOfs
		_ = readU32(l.Data) // recursionModelNameLen
		_ = readU32(l.Data) // recursionModelNameOfs
		entry.BlendingType = uint8(l.Data.ReadUInt8().(int64))
		entry.EmitterType = uint8(l.Data.ReadUInt8().(int64))
		entry.ParticleColorIndex = readU16(l.Data)
		_ = l.Data.ReadUInt8().(int64) // multiTextureParamX0
		_ = l.Data.ReadUInt8().(int64) // multiTextureParamX1
		_ = readU16(l.Data)            // textureTileRotation / priorityPlane
		entry.TextureRows = readU16(l.Data)
		entry.TextureCols = readU16(l.Data)
		entry.EmissionSpeed = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		entry.SpeedVariation = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		entry.VerticalRange = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		_ = ReadM2Track(l.Data, ofs, TrackFloat, nil) // horizontalRange
		entry.Gravity = ReadM2Track(l.Data, ofs, TrackUint32, nil)
		entry.Lifespan = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		entry.LifespanVary = l.Data.ReadFloatLE().(float32)
		entry.EmissionRate = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		_ = l.Data.ReadFloatLE().(float32) // emissionRateVary
		entry.EmissionAreaLength = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		entry.EmissionAreaWidth = ReadM2Track(l.Data, ofs, TrackFloat, nil)
		_ = ReadM2Track(l.Data, ofs, TrackFloat, nil) // zSource
		entry.ColorTrack = ReadM2PartTrack(l.Data, ofs, TrackFloat3)
		entry.AlphaTrack = ReadM2PartTrack(l.Data, ofs, TrackInt16)
		entry.ScaleTrack = ReadM2PartTrack(l.Data, ofs, TrackFloat2)
		entry.ScaleVary[0] = l.Data.ReadFloatLE().(float32)
		entry.ScaleVary[1] = l.Data.ReadFloatLE().(float32)
		entry.HeadCellTrack = ReadM2PartTrack(l.Data, ofs, TrackUint16)
		entry.TailCellTrack = ReadM2PartTrack(l.Data, ofs, TrackUint16)
		entry.TailLength = l.Data.ReadFloatLE().(float32)
		_ = l.Data.ReadFloatLE().(float32) // twinkleSpeed
		_ = l.Data.ReadFloatLE().(float32) // twinklePercent
		entry.TwinkleScale.Min = l.Data.ReadFloatLE().(float32)
		entry.TwinkleScale.Max = l.Data.ReadFloatLE().(float32)
		_ = l.Data.ReadFloatLE().(float32) // burstMultiplier
		entry.Drag = l.Data.ReadFloatLE().(float32)
		_ = l.Data.ReadFloatLE().(float32) // baseSpin
		_ = l.Data.ReadFloatLE().(float32) // baseSpinVary
		_ = l.Data.ReadFloatLE().(float32) // spin
		_ = l.Data.ReadFloatLE().(float32) // spinVary
		_ = readFloat3(l.Data)
		_ = readFloat3(l.Data)             // tumble min/max box
		_ = readFloat3(l.Data)             // windVector
		_ = l.Data.ReadFloatLE().(float32) // windTime
		_ = l.Data.ReadFloatLE().(float32) // followSpeed1
		_ = l.Data.ReadFloatLE().(float32) // followScale1
		_ = l.Data.ReadFloatLE().(float32) // followSpeed2
		_ = l.Data.ReadFloatLE().(float32) // followScale2
		_ = readU32(l.Data)                // splinePointsCount
		_ = readU32(l.Data)                // splinePointsOfs
		entry.EnabledIn = ReadM2Track(l.Data, ofs, TrackUint8, nil)
		tryReadParticleMultiTextureParams(l.Data)
		l.ParticleEmitters[i] = entry
	}
	l.Data.Seek(base)
}

func tryReadParticleMultiTextureParams(data *buffer.Buffer) {
	defer func() {
		_ = recover()
	}()
	_ = readFp69Vec2(data)
	_ = readFp69Vec2(data)
	_ = readFp69Vec2(data)
	_ = readFp69Vec2(data)
}

func readU32(b *buffer.Buffer) uint32 { return uint32(b.ReadUInt32LE().(int64)) }
func readU16(b *buffer.Buffer) uint16 { return uint16(b.ReadUInt16LE().(int64)) }

func readU32Slice(b *buffer.Buffer, count int) []uint32 {
	raw := b.ReadUInt32LE(count).([]int64)
	out := make([]uint32, count)
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}
