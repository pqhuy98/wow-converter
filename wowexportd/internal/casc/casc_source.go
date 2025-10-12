package casc

// Source is a minimal CASC interface to be used by server/core.
type Source interface {
	Init() error
	LoadBuild(idx int) error
	ListBuilds() []Build
	IsLoaded() bool
	GetBuildKey() string
	GetBuildName() string
	GetSelectedBuild() Build
	GetBuildConfigMap() map[string]string
	// ResolveEncodingKeyByFileID returns the encoding key for a given FileDataID
	ResolveEncodingKeyByFileID(fdid uint32) (string, error)
	// GetDataByEncodingKey fetches BLTE bytes for the given encoding key
	GetDataByEncodingKey(ekey string) ([]byte, error)
	// GetValidRootEntries returns FileDataIDs matching current locale and content filters
	GetValidRootEntries() []uint32
}

type Build struct {
	Product      string
	Region       string
	BuildConfig  string
	CDNConfig    string
	KeyRing      string
	BuildId      string
	VersionsName string
	Branch       string
	Version      string
	BuildKey     string
	CDNKey       string
}
