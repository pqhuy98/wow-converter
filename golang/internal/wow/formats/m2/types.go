package m2

// TextureEntry is an M2 texture reference.
type TextureEntry struct {
	FileDataID uint32
	Flags      uint32
	FileName   string
}

// MaterialEntry is an M2 material.
type MaterialEntry struct {
	Flags        uint16
	BlendingMode uint16
}

// BoneEntry is an M2 bone with animation tracks.
type BoneEntry struct {
	BoneID      int32
	Flags       uint32
	ParentBone  int16
	SubMeshID   uint16
	BoneNameCRC uint32
	Translation Track
	Rotation    Track
	Scale       Track
	Pivot       [3]float32
}

// AnimationEntry is an M2 animation definition.
type AnimationEntry struct {
	ID             uint16
	VariationIndex uint16
	Duration       uint32
	MoveSpeed      float32
	Flags          uint32
	Frequency      uint32
	ReplayMin      uint32
	ReplayMax      uint32
	BlendTimeIn    uint16
	BlendTimeOut   uint16
	BoxPosMin      [3]float32
	BoxPosMax      [3]float32
	BoxRadius      float32
	VariationNext  int16
	AliasNext      int16
}

// AttachmentEntry is an M2 attachment point.
type AttachmentEntry struct {
	ID       uint32
	Bone     uint16
	Unknown  uint16
	Position [3]float32
}

// ColorEntry is M2 vertex color animation data.
type ColorEntry struct {
	Color Track
	Alpha Track
}

// PartTrack is an age-based M2 particle track.
type PartTrack struct {
	Timestamps []uint16    `json:"timestamps"`
	Values     [][]float64 `json:"values"`
}

// CameraEntry is an M2 camera definition.
type CameraEntry struct {
	Type               int32      `json:"type"`
	FarClip            float32    `json:"far_clip"`
	NearClip           float32    `json:"near_clip"`
	PositionBase       [3]float32 `json:"position_base"`
	TargetPositionBase [3]float32 `json:"target_position_base"`
	FoV                Track      `json:"FoV"`
}

// LightEntry is an M2 light definition.
type LightEntry struct {
	Type             uint16     `json:"type"`
	Bone             int16      `json:"bone"`
	Position         [3]float32 `json:"position"`
	AmbientColor     Track      `json:"ambient_color"`
	AmbientIntensity Track      `json:"ambient_intensity"`
	DiffuseColor     Track      `json:"diffuse_color"`
	DiffuseIntensity Track      `json:"diffuse_intensity"`
	AttenuationStart Track      `json:"attenuation_start"`
	AttenuationEnd   Track      `json:"attenuation_end"`
	Visibility       Track      `json:"visibility"`
}

// RibbonEmitterEntry is an M2 ribbon emitter.
type RibbonEmitterEntry struct {
	RibbonID                    uint32     `json:"ribbonId"`
	BoneIndex                   uint32     `json:"boneIndex"`
	Position                    [3]float32 `json:"position"`
	TextureIndices              []uint16   `json:"textureIndices"`
	MaterialIndices             []uint16   `json:"materialIndices"`
	ColorTrack                  Track      `json:"colorTrack"`
	AlphaTrack                  Track      `json:"alphaTrack"`
	HeightAboveTrack            Track      `json:"heightAboveTrack"`
	HeightBelowTrack            Track      `json:"heightBelowTrack"`
	EdgesPerSecond              float32    `json:"edgesPerSecond"`
	EdgeLifetime                float32    `json:"edgeLifetime"`
	Gravity                     float32    `json:"gravity"`
	TextureRows                 uint16     `json:"textureRows"`
	TextureCols                 uint16     `json:"textureCols"`
	TexSlotTrack                Track      `json:"texSlotTrack"`
	VisibilityTrack             Track      `json:"visibilityTrack"`
	PriorityPlane               int16      `json:"priorityPlane"`
	RibbonColorIndex            int8       `json:"ribbonColorIndex"`
	TextureTransformLookupIndex int8       `json:"textureTransformLookupIndex"`
}

// ParticleEmitterEntry is an M2 particle emitter.
type ParticleEmitterEntry struct {
	ParticleID         uint32     `json:"particleId"`
	Flags              uint32     `json:"flags"`
	Position           [3]float32 `json:"position"`
	Bone               uint16     `json:"bone"`
	TexturePacked      uint16     `json:"texturePacked"`
	BlendingType       uint8      `json:"blendingType"`
	EmitterType        uint8      `json:"emitterType"`
	ParticleColorIndex uint16     `json:"particleColorIndex"`
	TextureRows        uint16     `json:"textureRows"`
	TextureCols        uint16     `json:"textureCols"`
	EmissionSpeed      Track      `json:"emissionSpeed"`
	SpeedVariation     Track      `json:"speedVariation"`
	VerticalRange      Track      `json:"verticalRange"`
	HorizontalRange    Track      `json:"horizontalRange"`
	Gravity            Track      `json:"gravity"`
	Lifespan           Track      `json:"lifespan"`
	LifespanVary       float32    `json:"lifespanVary"`
	EmissionRate       Track      `json:"emissionRate"`
	EmissionAreaLength Track      `json:"emissionAreaLength"`
	EmissionAreaWidth  Track      `json:"emissionAreaWidth"`
	ColorTrack         PartTrack  `json:"colorTrack"`
	AlphaTrack         PartTrack  `json:"alphaTrack"`
	ScaleTrack         PartTrack  `json:"scaleTrack"`
	ScaleVary          [2]float32 `json:"scaleVary"`
	HeadCellTrack      PartTrack  `json:"headCellTrack"`
	TailCellTrack      PartTrack  `json:"tailCellTrack"`
	TailLength         float32    `json:"tailLength"`
	TwinkleScale       struct {
		Min float32 `json:"min"`
		Max float32 `json:"max"`
	} `json:"twinkleScale"`
	Drag      float32 `json:"drag"`
	EnabledIn Track   `json:"enabledIn"`
}

// TextureTransformEntry is an M2 UV transform.
type TextureTransformEntry struct {
	Translation Track
	Rotation    Track
	Scaling     Track
}
