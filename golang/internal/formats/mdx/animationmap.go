package mdx

type animMapEntry struct {
	mdlName string
	kind    int
}

var animationMap = map[string]animMapEntry{
	// Layer
	"KMTF": {"TextureID", animValueUint},
	"KMTA": {"Alpha", animValueFloat},
	"KMTE": {"EmissiveGain", animValueFloat},
	"KFC3": {"FresnelColor", animValueVec3},
	"KFCA": {"FresnelOpacity", animValueFloat},
	"KFTC": {"FresnelTeamColor", animValueUint},
	// TextureAnimation
	"KTAT": {"Translation", animValueVec3},
	"KTAR": {"Rotation", animValueVec4},
	"KTAS": {"Scaling", animValueVec3},
	// GeosetAnimation
	"KGAO": {"Alpha", animValueFloat},
	"KGAC": {"Color", animValueVec3},
	// GenericObject
	"KGTR": {"Translation", animValueVec3},
	"KGRT": {"Rotation", animValueVec4},
	"KGSC": {"Scaling", animValueVec3},
	// Light
	"KLAS": {"AttenuationStart", animValueFloat},
	"KLAE": {"AttenuationEnd", animValueFloat},
	"KLAC": {"Color", animValueVec3},
	"KLAI": {"Intensity", animValueFloat},
	"KLBI": {"AmbIntensity", animValueFloat},
	"KLBC": {"AmbColor", animValueVec3},
	"KLAV": {"Visibility", animValueFloat},
	// Attachment
	"KATV": {"Visibility", animValueFloat},
	// ParticleEmitter
	"KPEE": {"EmissionRate", animValueFloat},
	"KPEG": {"Gravity", animValueFloat},
	"KPLN": {"Longitude", animValueFloat},
	"KPLT": {"Latitude", animValueFloat},
	"KPEL": {"LifeSpan", animValueFloat},
	"KPES": {"InitVelocity", animValueFloat},
	"KPEV": {"Visibility", animValueFloat},
	// ParticleEmitter2
	"KP2S": {"Speed", animValueFloat},
	"KP2R": {"Variation", animValueFloat},
	"KP2L": {"Latitude", animValueFloat},
	"KP2G": {"Gravity", animValueFloat},
	"KP2E": {"EmissionRate", animValueFloat},
	"KP2N": {"Width", animValueFloat},
	"KP2W": {"Length", animValueFloat},
	"KP2V": {"Visibility", animValueFloat},
	// ParticleEmitterPopcorn
	"KPPA": {"Alpha", animValueFloat},
	"KPPC": {"Color", animValueVec3},
	"KPPE": {"EmissionRate", animValueFloat},
	"KPPL": {"LifeSpan", animValueFloat},
	"KPPS": {"Speed", animValueFloat},
	"KPPV": {"Visibility", animValueFloat},
	// RibbonEmitter
	"KRHA": {"HeightAbove", animValueFloat},
	"KRHB": {"HeightBelow", animValueFloat},
	"KRAL": {"Alpha", animValueFloat},
	"KRCO": {"Color", animValueVec3},
	"KRTX": {"TextureSlot", animValueUint},
	"KRVS": {"Visibility", animValueFloat},
	// Camera
	"KCTR": {"Translation", animValueVec3},
	"KTTR": {"Translation", animValueVec3},
	"KCRL": {"Rotation", animValueFloat},
}
