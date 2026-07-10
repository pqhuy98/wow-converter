package db

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

var (
	patternColumn    = regexp.MustCompile(`^(int|float|locstring|string)(<[^:]+::[^>]+>)?\s([^\s]+)`)
	patternBuild     = regexp.MustCompile(`^BUILD\s(.*)`)
	patternBuildRange = regexp.MustCompile(`([^-]+)-(.*)`)
	patternComment   = regexp.MustCompile(`^COMMENT\s`)
	patternLayout    = regexp.MustCompile(`^LAYOUT\s(.*)`)
	patternField     = regexp.MustCompile(`^(\$([^$]+)\$)?([^<[]+)(<(u|)(\d+)>)?(\[(\d+)\])?$`)
	patternBuildID   = regexp.MustCompile(`(\d+).(\d+).(\d+).(\d+)`)
)

// DBDColumnType is a column type from a DBD COLUMNS section.
type DBDColumnType string

const (
	DBDColumnInt       DBDColumnType = "int"
	DBDColumnFloat     DBDColumnType = "float"
	DBDColumnLocString DBDColumnType = "locstring"
	DBDColumnString    DBDColumnType = "string"
)

type buildID struct {
	major, minor, patch, rev int
}

func parseBuildID(build string) buildID {
	parts := patternBuildID.FindStringSubmatch(build)
	entry := buildID{}
	if parts != nil {
		entry.major, _ = strconv.Atoi(parts[1])
		entry.minor, _ = strconv.Atoi(parts[2])
		entry.patch, _ = strconv.Atoi(parts[3])
		entry.rev, _ = strconv.Atoi(parts[4])
	}
	return entry
}

func isBuildInRange(buildStr, minStr, maxStr string) bool {
	build := parseBuildID(buildStr)
	min := parseBuildID(minStr)
	max := parseBuildID(maxStr)
	if build.major < min.major || build.major > max.major {
		return false
	}
	if build.minor < min.minor || build.minor > max.minor {
		return false
	}
	if build.patch < min.patch || build.patch > max.patch {
		return false
	}
	if build.rev < min.rev || build.rev > max.rev {
		return false
	}
	return true
}

// DBDField describes a field in a DBD entry.
type DBDField struct {
	Type        DBDColumnType
	Name        string
	IsSigned    bool
	IsID        bool
	IsInline    bool
	IsRelation  bool
	ArrayLength int
	Size        int
}

// DBDEntry is a version-specific table definition from a DBD file.
type DBDEntry struct {
	Builds      map[string]struct{}
	BuildRanges []struct{ Min, Max string }
	LayoutHashes map[string]struct{}
	Fields      []*DBDField
}

func newDBDEntry() *DBDEntry {
	return &DBDEntry{
		Builds:       make(map[string]struct{}),
		LayoutHashes: make(map[string]struct{}),
	}
}

func (e *DBDEntry) addBuild(min string, max ...string) {
	if len(max) > 0 {
		e.BuildRanges = append(e.BuildRanges, struct{ Min, Max string }{min, max[0]})
	} else {
		e.Builds[min] = struct{}{}
	}
}

func (e *DBDEntry) addLayoutHashes(hashes ...string) {
	for _, h := range hashes {
		e.LayoutHashes[strings.TrimSpace(h)] = struct{}{}
	}
}

func (e *DBDEntry) addField(field *DBDField) {
	e.Fields = append(e.Fields, field)
}

// IsValidFor reports whether this entry matches buildID or layoutHash.
func (e *DBDEntry) IsValidFor(buildID, layoutHash string) bool {
	if _, ok := e.LayoutHashes[layoutHash]; ok {
		return true
	}
	if _, ok := e.Builds[buildID]; ok {
		return true
	}
	for _, r := range e.BuildRanges {
		if isBuildInRange(buildID, r.Min, r.Max) {
			return true
		}
	}
	return false
}

// DBDParser parses DBD definition files.
type DBDParser struct {
	Entries []*DBDEntry
	columns map[string]DBDColumnType
}

// NewDBDParser parses DBD data from a buffer.
func NewDBDParser(data *buffer.Buffer) (*DBDParser, error) {
	p := &DBDParser{columns: make(map[string]DBDColumnType)}
	if err := p.parse(data); err != nil {
		return nil, err
	}
	return p, nil
}

// GetStructure returns the matching DBD entry for build/layout.
func (p *DBDParser) GetStructure(buildID, layoutHash string) *DBDEntry {
	for _, entry := range p.Entries {
		if entry.IsValidFor(buildID, layoutHash) {
			return entry
		}
	}
	return nil
}

func (p *DBDParser) parse(data *buffer.Buffer) error {
	lines := data.ReadLines("utf8")
	var chunk []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			chunk = append(chunk, line)
		} else {
			if err := p.parseChunk(chunk); err != nil {
				return err
			}
			chunk = nil
		}
	}
	if len(chunk) > 0 {
		if err := p.parseChunk(chunk); err != nil {
			return err
		}
	}
	if len(p.columns) == 0 {
		return fmt.Errorf("invalid DBD: no columns defined")
	}
	return nil
}

func (p *DBDParser) parseChunk(chunk []string) error {
	if len(chunk) == 0 {
		return nil
	}
	if chunk[0] == "COLUMNS" {
		return p.parseColumnChunk(chunk)
	}
	entry := newDBDEntry()
	for _, line := range chunk {
		if buildMatch := patternBuild.FindStringSubmatch(line); buildMatch != nil {
			for _, build := range strings.Split(buildMatch[1], ",") {
				build = strings.TrimSpace(build)
				if buildRange := patternBuildRange.FindStringSubmatch(build); buildRange != nil {
					entry.addBuild(buildRange[1], buildRange[2])
				} else {
					entry.addBuild(build)
				}
			}
			continue
		}
		if patternComment.MatchString(line) {
			continue
		}
		if layoutMatch := patternLayout.FindStringSubmatch(line); layoutMatch != nil {
			entry.addLayoutHashes(strings.Split(layoutMatch[1], ",")...)
			continue
		}
		if fieldMatch := patternField.FindStringSubmatch(line); fieldMatch != nil {
			fieldName := fieldMatch[3]
			fieldType, ok := p.columns[fieldName]
			if !ok {
				return fmt.Errorf("invalid DBD: no field type defined for %s", fieldName)
			}
			field := &DBDField{
				Type:        fieldType,
				Name:        fieldName,
				IsSigned:    true,
				IsInline:    true,
				ArrayLength: -1,
				Size:        -1,
			}
			if fieldMatch[2] != "" {
				for _, ann := range strings.Split(fieldMatch[2], ",") {
					switch ann {
					case "id":
						field.IsID = true
					case "noninline":
						field.IsInline = false
					case "relation":
						field.IsRelation = true
					}
				}
			}
			if fieldMatch[5] != "" {
				field.IsSigned = false
			}
			if fieldMatch[6] != "" {
				if size, err := strconv.Atoi(fieldMatch[6]); err == nil {
					field.Size = size
				}
			}
			if fieldMatch[8] != "" {
				if n, err := strconv.Atoi(fieldMatch[8]); err == nil {
					field.ArrayLength = n
				}
			}
			entry.addField(field)
		}
	}
	p.Entries = append(p.Entries, entry)
	return nil
}

func (p *DBDParser) parseColumnChunk(chunk []string) error {
	if len(chunk) == 0 {
		return fmt.Errorf("invalid DBD: missing column definitions")
	}
	for _, entry := range chunk[1:] {
		if match := patternColumn.FindStringSubmatch(entry); match != nil {
			columnName := strings.ReplaceAll(match[3], "?", "")
			p.columns[columnName] = DBDColumnType(match[1])
		}
	}
	return nil
}
