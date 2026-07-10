package data

// ScriptLanguage identifies the map script language.
type ScriptLanguage int

const (
	ScriptLanguageJASS ScriptLanguage = 0
	ScriptLanguageLua  ScriptLanguage = 1
)

// SupportedModes identifies SD/HD support flags.
type SupportedModes int

const (
	SupportedModesSD   SupportedModes = 1
	SupportedModesHD   SupportedModes = 2
	SupportedModesBoth SupportedModes = 3
)

// Availability is upgrade availability state.
type Availability int

const (
	AvailabilityUnavailable Availability = 0
	AvailabilityAvailable     Availability = 1
	AvailabilityResearched    Availability = 2
)

// MapInfo is war3map.w3i map metadata.
type MapInfo struct {
	Name               string
	Author             string
	Description        string
	RecommendedPlayers string
	PlayableArea       PlayableMapArea
	Flags              MapFlags
	MainTileType       string
}

// PlayableMapArea is the playable map size in tiles.
type PlayableMapArea struct {
	Width  int
	Height int
}

// MapFlags are war3map.w3i map property flags.
type MapFlags struct {
	HideMinimapInPreview                                  bool
	ModifyAllyPriorities                                  bool
	IsMeleeMap                                            bool
	NonDefaultTilesetMapSizeLargeNeverBeenReducedToMedium bool
	MaskedPartiallyVisible                                bool
	FixedPlayerSetting                                    bool
	UseCustomForces                                       bool
	UseCustomTechtree                                     bool
	UseCustomAbilities                                    bool
	UseCustomUpgrades                                     bool
	MapPropertiesMenuOpenedAtLeastOnce                    bool
	WaterWavesOnCliffShores                               bool
	WaterWavesOnRollingShores                             bool
	UseTerrainFog                                         bool
	TftRequired                                           bool
	UseItemClassificationSystem                           bool
	EnableWaterTinting                                    bool
	UseAccurateProbabilityForCalculations                 bool
	UseCustomAbilitySkins                                 bool
}

// GameVersion is the WC3 game version stored in w3i.
type GameVersion struct {
	Major int
	Minor int
	Patch int
	Build int
}

// PlayableCamera is camera bounds/complements from w3i.
type PlayableCamera struct {
	Bounds      [8]float32
	Complements [4]int32
}

// LoadingScreen is the map loading screen configuration.
type LoadingScreen struct {
	Background int
	Path       string
	Text       string
	Title      string
	Subtitle   string
}

// Fog is terrain fog settings.
type Fog struct {
	Type        int
	StartHeight float32
	EndHeight   float32
	Density     float32
	Color       [4]byte
}

// Prologue is the prologue screen configuration.
type Prologue struct {
	Path     string
	Text     string
	Title    string
	Subtitle string
}

// PlayerStartingPosition is a player's start location.
type PlayerStartingPosition struct {
	X     float32
	Y     float32
	Fixed bool
}

// Player is a map slot player definition.
type Player struct {
	PlayerNum            int
	Type                 int
	Race                 int
	Name                 string
	StartingPos          PlayerStartingPosition
	AllyLowPriorities    int32
	AllyHighPriorities   int32
	EnemyLowPriorities   int32
	EnemyHighPriorities  int32
}

// ForceFlags are custom force settings.
type ForceFlags struct {
	Allied             bool
	AlliedVictory      bool
	ShareVision        bool
	ShareUnitControl   bool
	ShareAdvUnitControl bool
}

// Force is a custom force definition.
type Force struct {
	Flags   ForceFlags
	Players int32
	Name    string
}

// UpgradeAvailable is an upgrade availability override.
type UpgradeAvailable struct {
	PlayerFlags  int32
	UpgradeID    string
	Level        int32
	Availability int32
}

// TechUnavailable is a tech blacklist entry.
type TechUnavailable struct {
	PlayerFlags int32
	TechID      string
}

// RandomUnitChance is one row in a random unit table.
type RandomUnitChance struct {
	Chance  int32
	UnitIDs []string
}

// RandomUnitTable is a random unit group from w3i.
type RandomUnitTable struct {
	ID        int32
	Name      string
	Positions []int32
	Chances   []RandomUnitChance
}

// RandomObject is an object in a random item pool.
type RandomObject struct {
	ObjectID string
	Chance   int32
}

// ObjectPool is a row in a random item table.
type ObjectPool struct {
	Type    int
	Objects []RandomObject
}

// RandomItemTable is a random item group from w3i.
type RandomItemTable struct {
	ID   int32
	Name string
	Rows []ObjectPool
}

// Info is war3map.w3i map metadata.
type Info struct {
	FileVersion       int32
	Saves             int32
	EditorVersion     int32
	GameVersion       GameVersion
	Map               MapInfo
	Camera            PlayableCamera
	GameDataSet       int32
	Prologue          Prologue
	LoadingScreen     LoadingScreen
	Fog               Fog
	GlobalWeather     int32
	CustomSoundEnv    string
	CustomLightEnv    byte
	Water             [4]byte
	ScriptLanguage    ScriptLanguage
	SupportedModes    SupportedModes
	GameDataVersion   int32
	DefaultCameraZoom int32
	MaxCameraZoom     int32
	MinCameraZoom     int32
	Players           []Player
	Forces            []Force
	Upgrades          []UpgradeAvailable
	TechBlacklist     []TechUnavailable
	RandomUnitTables  []RandomUnitTable
	RandomItemTables  []RandomItemTable
}
