package character

import (
	"context"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// Ref identifies a model source.
type Ref struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// AttachItem describes an item attached to a bone.
type AttachItem struct {
	Path  Ref     `json:"path"`
	Scale float64 `json:"scale,omitempty"`
}

// Mount describes a mount configuration.
type Mount struct {
	Path       Ref       `json:"path"`
	Scale      float64   `json:"scale,omitempty"`
	SeatOffset []float64 `json:"seatOffset,omitempty"`
	Animation  string    `json:"animation,omitempty"`
}

// Character holds export configuration for a WoW character or creature.
type Character struct {
	Base                       Ref                    `json:"base"`
	AttackTag                  animmap.AttackTag      `json:"attackTag,omitempty"`
	KeepCinematic              bool                   `json:"keepCinematic,omitempty"`
	InGameMovespeed            float64                `json:"inGameMovespeed"`
	Size                       string                 `json:"size,omitempty"`
	Scale                      float64                `json:"scale,omitempty"`
	AttachItems                map[string]AttachItem  `json:"attachItems,omitempty"`
	NoDecay                    bool                   `json:"noDecay,omitempty"`
	ParticlesDensity           *float64               `json:"particlesDensity,omitempty"`
	PortraitCameraSequenceName string                 `json:"portraitCameraSequenceName,omitempty"`
	Mount                      *Mount                 `json:"mount,omitempty"`
	ForceSheathed              bool                   `json:"forceSheathed,omitempty"`
	WithCollision              bool                   `json:"withCollision,omitempty"`
}

// LocalRef creates a local file ref.
func LocalRef(path string) Ref { return Ref{Type: "local", Value: path} }

// WowheadRef creates a wowhead URL ref.
func WowheadRef(url string) Ref { return Ref{Type: "wowhead", Value: url} }

// DisplayRef creates a display ID ref.
func DisplayRef(id int) Ref { return Ref{Type: "displayID", Value: itoa(id)} }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

// ExportContext carries per-export state.
type ExportContext struct {
	AssetManager        *common.AssetManager
	Config              config.Config
	OutputFile          string
	WowClient           client.Client
	Wowhead             WowheadClient
	HTTP                *wowhead.HTTPClient
	WeaponInventoryTypes [2]*int
	ForceSheathed       bool
	WithCollision       bool
	LocalModelSkinID    string
}

// WowheadHTTP returns the wowhead HTTP client for meta fetches.
func (ctx *ExportContext) WowheadHTTP() *wowhead.HTTPClient {
	if ctx.HTTP != nil {
		return ctx.HTTP
	}
	return wowhead.NewHTTPClient()
}

// IsClassicCASC reports whether the loaded CASC product is a Classic variant.
func (ctx *ExportContext) IsClassicCASC(c context.Context) bool {
	if ctx.WowClient == nil {
		return false
	}
	info, err := ctx.WowClient.GetCASCInfo(c)
	if err != nil || info.BuildName == "" {
		return false
	}
	return client.IsClassicProduct(info.Build.Product)
}

// ResolveMetaExpansion picks the wowhead/zam expansion for meta fetches.
// Retail CASC installs cannot resolve classic-era assets, so URL expansion is
// ignored unless the loaded product is a Classic variant (TS parity).
func (ctx *ExportContext) ResolveMetaExpansion(urlExpansion wowhead.Expansion) wowhead.Expansion {
	if ctx.WowClient == nil {
		return urlExpansion
	}
	info, err := ctx.WowClient.GetCASCInfo(context.Background())
	if err != nil || info.BuildName == "" {
		return urlExpansion
	}
	if client.IsClassicProduct(info.Build.Product) {
		if urlExpansion == wowhead.ExpansionLatestAvailable {
			return classicWowheadExpansion(info.Build.Version, info.BuildName)
		}
		return urlExpansion
	}
	if urlExpansion == wowhead.ExpansionLatestAvailable {
		return wowhead.ExpansionLatestAvailable
	}
	return wowhead.ExpansionLatestAvailable
}

func classicWowheadExpansion(version, buildName string) wowhead.Expansion {
	ver := version
	if ver == "" {
		ver = buildName
	}
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) < 2 {
		return wowhead.ExpansionWrath
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return wowhead.ExpansionWrath
	}
	switch {
	case major >= 11:
		return wowhead.ExpansionLive
	case major == 5:
		switch {
		case minor >= 5:
			return wowhead.ExpansionMists
		case minor >= 4:
			return wowhead.ExpansionCata
		default:
			return wowhead.ExpansionWrath
		}
	case major == 4:
		return wowhead.ExpansionCata
	case major == 3:
		return wowhead.ExpansionWrath
	case major == 2:
		return wowhead.ExpansionTBC
	default:
		return wowhead.ExpansionClassic
	}
}

// WowheadClient fetches wowhead/zam metadata.
type WowheadClient interface {
	FetchNpcMeta(ctx context.Context, url ZamURL) (CharacterData, error)
}

// ZamURL is re-exported shape for wowhead client.
type ZamURL struct {
	Expansion string
	Type      string
	DisplayID int
}

// CharacterData is creature/character meta from wowhead.
type CharacterData struct {
	Model        *int
	Textures     map[string]int
	Character    *wowhead.CharacterMeta
	Creature     *CreatureMeta
	Equipment    map[string]int
	TextureFiles map[string][]wowhead.FileEntry
	ItemEffects  []wowhead.ItemEffect
}

// CreatureMeta holds creature geoset data.
type CreatureMeta struct {
	CreatureGeosetData       []GeosetEntry
	CreatureCustomizations   []wowhead.Customization
}

// GeosetEntry is a creature geoset selection.
type GeosetEntry struct {
	GeosetIndex int
	GeosetValue int
}

// CascFileSource adapts wow client to direct M2 file source.
type CascFileSource struct {
	Client client.Client
}

func (s CascFileSource) GetRawFile(ctx context.Context, fileDataID int) ([]byte, error) {
	return s.Client.DownloadCascFile(ctx, fileDataID)
}

func (s CascFileSource) GetFileName(ctx context.Context, fileDataID int) (string, error) {
	entry, err := s.Client.GetFileByID(ctx, fileDataID)
	if err != nil {
		return "", err
	}
	return entry.FileName, nil
}

func (s CascFileSource) GetModelSkins(ctx context.Context, fileDataID int) ([]directm2.ModelSkin, error) {
	skins, err := s.Client.GetModelSkins(ctx, fileDataID)
	if err != nil {
		return nil, err
	}
	out := make([]directm2.ModelSkin, len(skins))
	for i, sk := range skins {
		out[i] = directm2.ModelSkin{ID: sk.ID, ExtraGeosets: sk.ExtraGeosets, Textures: sk.Textures}
	}
	return out, nil
}

func (s CascFileSource) GetBuildKey(ctx context.Context) (string, error) {
	if s.Client == nil {
		return "", nil
	}
	info, err := s.Client.GetCASCInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.BuildKey, nil
}
