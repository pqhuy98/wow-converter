package casc

const installSig = 0x4E49

// InstallTag describes an install manifest tag.
type InstallTag struct {
	Name string
	Type int
	Mask []byte
}

// InstallFile describes an install manifest file entry.
type InstallFile struct {
	Name string
	Hash string
	Size int
	Tags []string
}

// InstallManifest parses install manifest data.
type InstallManifest struct {
	Version  int
	HashSize int
	NumTags  int
	NumFiles int
	MaskSize int
	Tags     []InstallTag
	Files    []InstallFile
}

// NewInstallManifest parses an install manifest from BLTE-decoded data.
func NewInstallManifest(data interface {
	ReadUInt16LE(count ...int) any
	ReadUInt8(count ...int) any
	ReadUInt16BE(count ...int) any
	ReadUInt32BE(count ...int) any
	ReadNullTerminatedString(encoding string) string
	ReadHexString(length int) string
}) (*InstallManifest, error) {
	m := &InstallManifest{}
	if err := m.Parse(data); err != nil {
		return nil, err
	}
	return m, nil
}

// Parse parses install manifest data.
func (m *InstallManifest) Parse(data interface {
	ReadUInt16LE(count ...int) any
	ReadUInt8(count ...int) any
	ReadUInt16BE(count ...int) any
	ReadUInt32BE(count ...int) any
	ReadNullTerminatedString(encoding string) string
	ReadHexString(length int) string
}) error {
	if data.ReadUInt16LE().(int64) != installSig {
		return errInvalidInstallManifest
	}
	m.Version = int(data.ReadUInt8().(int64))
	m.HashSize = int(data.ReadUInt8().(int64))
	m.NumTags = int(data.ReadUInt16BE().(int64))
	m.NumFiles = int(data.ReadUInt32BE().(int64))
	m.Tags = make([]InstallTag, m.NumTags)
	m.Files = make([]InstallFile, m.NumFiles)
	m.MaskSize = (m.NumFiles + 7) / 8

	for i := 0; i < m.NumTags; i++ {
		m.Tags[i] = InstallTag{
			Name: data.ReadNullTerminatedString("utf8"),
			Type: int(data.ReadUInt16BE().(int64)),
		}
		mask := data.ReadUInt8(m.MaskSize).([]int64)
		maskBytes := make([]byte, len(mask))
		for j, v := range mask {
			maskBytes[j] = byte(v)
		}
		m.Tags[i].Mask = maskBytes
	}

	for i := 0; i < m.NumFiles; i++ {
		m.Files[i] = InstallFile{
			Name: data.ReadNullTerminatedString("utf8"),
			Hash: data.ReadHexString(m.HashSize),
			Size: int(data.ReadUInt32BE().(int64)),
			Tags: []string{},
		}
	}

	for _, tag := range m.Tags {
		mask := tag.Mask
		n := len(mask)
		for i := 0; i < n; i++ {
			for j := 0; j < 8; j++ {
				if ((mask[i] >> (7 - j)) & 0x1) == 1 {
					idx := ((i % n) * 8) + j
					if idx < len(m.Files) {
						m.Files[idx].Tags = append(m.Files[idx].Tags, tag.Name)
					}
				}
			}
		}
	}
	return nil
}
