package m2

import (
	"context"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

const (
	chunkSKB1 = 0x31424B53
	chunkSKPD = 0x44504B53
	chunkSKS1 = 0x31534B53
	chunkSKA1 = 0x31414B53
)

// SkelLoader parses WoW .skel skeleton files.
type SkelLoader struct {
	Data    *buffer.Buffer
	getFile func(ctx context.Context, fileDataID uint32) ([]byte, error)

	IsLoaded         bool
	AnimFiles        map[int]*buffer.Buffer
	AnimFileIDs      []AnimFileIDEntry
	ParentSkelFileID uint32
	BoneOffset       int
	Bones            []BoneEntry
	Animations       []AnimationEntry
	Attachments      []AttachmentEntry
}

// NewSkelLoader creates a .skel loader.
func NewSkelLoader(data *buffer.Buffer, getFile func(context.Context, uint32) ([]byte, error)) *SkelLoader {
	return &SkelLoader{
		Data:      data,
		getFile:   getFile,
		AnimFiles: map[int]*buffer.Buffer{},
	}
}

// Load parses the skeleton file.
func (l *SkelLoader) Load() {
	if l.IsLoaded {
		return
	}
	for l.Data.RemainingBytes() > 0 {
		chunkID := readU32(l.Data)
		chunkSize := int(readU32(l.Data))
		nextChunkPos := l.Data.Offset() + chunkSize
		switch chunkID {
		case chunkSKA1:
			l.parseChunkSKA1(false)
		case chunkSKB1:
			l.parseChunkSKB1(false)
		case chunkSKPD:
			l.parseChunkSKPD()
		case chunkSKS1:
			l.parseChunkSKS1()
		case chunkAFID:
			l.parseChunkAFID(chunkSize)
		case chunkBFID:
			_ = readU32Slice(l.Data, chunkSize/4)
		}
		l.Data.Seek(nextChunkPos)
	}
	l.IsLoaded = true
}

func (l *SkelLoader) parseChunkSKPD() {
	l.Data.Move(8)
	l.ParentSkelFileID = readU32(l.Data)
	l.Data.Move(4)
}

func (l *SkelLoader) parseChunkSKB1(useAnims bool) {
	data := l.Data
	chunkOfs := data.Offset()
	l.BoneOffset = chunkOfs

	boneCount := int(readU32(data))
	boneOfs := int(readU32(data))
	baseOfs := data.Offset()
	data.Seek(chunkOfs + boneOfs)

	animFiles := l.animFilesFor(useAnims)
	l.Bones = make([]BoneEntry, boneCount)
	for i := 0; i < boneCount; i++ {
		bone := BoneEntry{
			BoneID: int32(readU32(data)), Flags: readU32(data),
			ParentBone: int16(readU16(data)), SubMeshID: readU16(data),
			BoneNameCRC: readU32(data),
			Translation: ReadM2Track(data, chunkOfs, TrackFloat3, animFiles),
			Rotation:    ReadM2Track(data, chunkOfs, TrackCompQuat, animFiles),
			Scale:       ReadM2Track(data, chunkOfs, TrackFloat3, animFiles),
			Pivot:       readFloat3(data),
		}
		convertBoneCoords(&bone)
		l.Bones[i] = bone
	}
	data.Seek(baseOfs)
}

func (l *SkelLoader) animFilesFor(useAnims bool) map[int]*buffer.Buffer {
	if useAnims {
		return l.AnimFiles
	}
	return nil
}

func (l *SkelLoader) parseChunkSKS1() {
	chunkOfs := l.Data.Offset()

	globalLoopCount := int(readU32(l.Data))
	globalLoopOfs := int(readU32(l.Data))
	prevPos := l.Data.Offset()
	l.Data.Seek(globalLoopOfs + chunkOfs)
	_ = readU16Slice(l.Data, globalLoopCount)
	l.Data.Seek(prevPos)

	animationCount := int(readU32(l.Data))
	animationOfs := int(readU32(l.Data))
	prevPos = l.Data.Offset()
	l.Data.Seek(animationOfs + chunkOfs)

	l.Animations = make([]AnimationEntry, animationCount)
	for i := 0; i < animationCount; i++ {
		entry := AnimationEntry{
			ID: readU16(l.Data), VariationIndex: readU16(l.Data),
			Duration: readU32(l.Data),
			MoveSpeed: l.Data.ReadFloatLE().(float32),
			Flags: readU32(l.Data),
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
	l.Data.Seek(prevPos)

	animationLookupCount := int(readU32(l.Data))
	animationLookupOfs := int(readU32(l.Data))
	prevPos = l.Data.Offset()
	l.Data.Seek(animationLookupOfs + chunkOfs)
	_ = readU16Slice(l.Data, animationLookupCount)
	l.Data.Seek(prevPos)
	l.Data.Move(8)
}

func (l *SkelLoader) parseChunkSKA1(useAnims bool) {
	data := l.Data
	chunkOfs := data.Offset()

	attachmentCount := int(readU32(data))
	attachmentOfs := int(readU32(data))
	lookupCount := int(readU32(data))
	lookupOfs := int(readU32(data))
	base := data.Offset()

	if attachmentCount > 0 && attachmentOfs > 0 {
		data.Seek(chunkOfs + attachmentOfs)
		l.Attachments = make([]AttachmentEntry, attachmentCount)
		for i := 0; i < attachmentCount; i++ {
			att := AttachmentEntry{
				ID: readU32(data), Bone: readU16(data), Unknown: readU16(data),
				Position: readFloat3(data),
			}
			pos := att.Position
			att.Position = [3]float32{pos[0], pos[2], -pos[1]}
			_ = ReadM2Track(data, chunkOfs, TrackUint8, l.animFilesFor(useAnims))
			l.Attachments[i] = att
		}
	}
	if lookupCount > 0 && lookupOfs > 0 {
		data.Seek(chunkOfs + lookupOfs)
		_ = readU16Slice(data, lookupCount)
	}
	data.Seek(base)
}

func (l *SkelLoader) parseChunkAFID(chunkSize int) {
	entryCount := chunkSize / 8
	l.AnimFileIDs = make([]AnimFileIDEntry, entryCount)
	for i := 0; i < entryCount; i++ {
		l.AnimFileIDs[i] = AnimFileIDEntry{
			AnimID: readU16(l.Data), SubAnimID: readU16(l.Data),
			FileDataID: readU32(l.Data),
		}
	}
}

// LoadAnims loads external .anim files and re-parses bone tracks.
func (l *SkelLoader) LoadAnims(ctx context.Context) error {
	for i := range l.Animations {
		if err := l.LoadAnimsForIndex(ctx, i, false); err != nil {
			return err
		}
	}
	l.Data.Seek(l.BoneOffset)
	l.parseChunkSKB1(true)
	return nil
}

// LoadAnimsForIndex loads a single animation's .anim file.
func (l *SkelLoader) LoadAnimsForIndex(ctx context.Context, animationIndex int, reparseBones bool) error {
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
		animLoader := NewAnimLoader(buffer.From(raw))
		animLoader.Load(true)
		if animLoader.SkeletonBoneData != nil {
			l.AnimFiles[animationIndex] = buffer.From(animLoader.SkeletonBoneData)
		} else if animLoader.AnimData != nil {
			l.AnimFiles[animationIndex] = buffer.From(animLoader.AnimData)
		}
		if reparseBones {
			l.Data.Seek(l.BoneOffset)
			l.parseChunkSKB1(true)
		}
		return nil
	}
	return nil
}
