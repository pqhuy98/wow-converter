package casc

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Listfile mirrors wow.export listfile API naming (getByID, getByFilename, etc.).
type Listfile struct {
	mu       sync.RWMutex
	idToName map[uint32]string
	nameToID map[string]uint32
	isLoaded bool
}

func NewListfile() *Listfile {
	return &Listfile{idToName: make(map[uint32]string), nameToID: make(map[string]uint32)}
}

func (s *Listfile) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	idToName := make(map[uint32]string)
	nameToID := make(map[string]uint32)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ";")
		if len(parts) != 2 {
			continue
		}
		// parse numeric ID quickly
		v := uint64(0)
		for i := 0; i < len(parts[0]); i++ {
			c := parts[0][i]
			if c < '0' || c > '9' {
				v = 0
				break
			}
			v = v*10 + uint64(c-'0')
		}
		name := strings.ToLower(parts[1])
		idToName[uint32(v)] = name
		nameToID[name] = uint32(v)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.idToName = idToName
	s.nameToID = nameToID
	s.isLoaded = true
	s.mu.Unlock()
	return nil
}

func (s *Listfile) LoadFromDefaultCache() error {
	p := filepath.Join(DirListfile(), "listfile.txt")
	if _, err := os.Stat(p); err == nil {
		return s.LoadFromFile(p)
	}
	return os.ErrNotExist
}

func (s *Listfile) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isLoaded
}

// GetByID returns the filename for a FileDataID.
func (s *Listfile) GetByID(id uint32) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idToName[id]
}

// GetByFilename returns the FileDataID for a given filename (case-insensitive, path-normalized).
func (s *Listfile) GetByFilename(name string) uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	norm := strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	if id, ok := s.nameToID[norm]; ok {
		return id
	}
	// MDL/MDX fallback to M2
	if strings.HasSuffix(norm, ".mdl") || strings.HasSuffix(norm, "mdx") {
		base := norm
		if ext := filepath.Ext(norm); ext != "" {
			base = norm[:len(norm)-len(ext)]
		}
		repl := base + ".m2"
		if id, ok := s.nameToID[repl]; ok {
			return id
		}
	}
	return 0
}

// GetFilteredEntries mirrors wow.export getFilteredEntries output shape.
func (s *Listfile) GetFilteredEntries(search any) []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]map[string]any, 0, 1024)
	var re *regexp.Regexp
	var substr string
	switch v := search.(type) {
	case *regexp.Regexp:
		re = v
	case string:
		substr = strings.ToLower(v)
	}
	for id, name := range s.idToName {
		if re != nil {
			if re.MatchString(name) {
				entries = append(entries, map[string]any{"fileDataID": id, "fileName": name})
			}
		} else if substr == "" || strings.Contains(name, substr) {
			entries = append(entries, map[string]any{"fileDataID": id, "fileName": name})
		}
	}
	// Maintain insertion order parity with JS; do not sort here
	return entries
}

// AddEntry adds a mapping to the listfile.
func (s *Listfile) AddEntry(id uint32, name string) {
	s.mu.Lock()
	s.idToName[id] = name
	s.nameToID[strings.ToLower(strings.ReplaceAll(name, "\\", "/"))] = id
	s.mu.Unlock()
}

// ApplyRootFilter trims the listfile to only include provided FileDataIDs.
func (s *Listfile) ApplyRootFilter(valid []uint32) {
	if len(valid) == 0 {
		return
	}
	s.mu.Lock()
	keep := make(map[uint32]struct{}, len(valid))
	for _, id := range valid {
		keep[id] = struct{}{}
	}
	for id, name := range s.idToName {
		if _, ok := keep[id]; !ok {
			delete(s.nameToID, name)
			delete(s.idToName, id)
		}
	}
	s.mu.Unlock()
}

// LoadIDTable adds unknown entries for the given IDs with the specified extension
// if they are not already present. Returns the number of entries added.
// Mirrors wow.export listfile.loadIDTable behavior: 'unknown/<id><ext>'.
func (s *Listfile) LoadIDTable(ids []uint32, ext string) int {
	added := 0
	s.mu.Lock()
	for _, id := range ids {
		if _, ok := s.idToName[id]; ok {
			continue
		}
		name := "unknown/" + fmtUint32(id) + ext
		s.idToName[id] = name
		s.nameToID[name] = id
		added++
	}
	s.mu.Unlock()
	return added
}

// LoadUnknownTextures convenience wrapper for .blp unknowns.
func (s *Listfile) LoadUnknownTextures(ids []uint32) int { return s.LoadIDTable(ids, ".blp") }

// LoadUnknownModels convenience wrapper for .m2 unknowns.
func (s *Listfile) LoadUnknownModels(ids []uint32) int { return s.LoadIDTable(ids, ".m2") }

// fmtUint32 fast zero-allocation conversion for positive uint32 values
func fmtUint32(v uint32) string {
	// up to 10 digits
	var buf [10]byte
	i := len(buf)
	n := v
	for n >= 10 {
		q := n / 10
		r := n - q*10
		i--
		buf[i] = byte('0' + r)
		n = q
	}
	i--
	buf[i] = byte('0' + n)
	return string(buf[i:])
}
