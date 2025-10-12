package resthandlers

import (
	"log"
	"wowexportd/internal/casc"
)

// ingestUnknowns scans valid root entries and adds unknown texture/model entries to listfile.
// It detects formats by reading BLTE and checking file magic. Limit <= 0 means no limit.
func ingestUnknowns(state cascState, list Listfile, limit int) {
	if list == nil || !list.IsLoaded() {
		return
	}
	s := state.GetActive()
	if s == nil || !s.IsLoaded() {
		return
	}
	ids := s.GetValidRootEntries()
	added := 0
	for _, id := range ids {
		if limit > 0 && added >= limit {
			break
		}
		if name := list.GetByID(id); name != "" {
			continue
		}
		ekey, err := s.ResolveEncodingKeyByFileID(id)
		if err != nil || ekey == "" {
			continue
		}
		data, err := s.GetDataByEncodingKey(ekey)
		if err != nil || len(data) < 8 {
			continue
		}
		br, err := casc.NewBLTEReader(data, ekey, true)
		if err != nil {
			continue
		}
		out := br.Output()
		if len(out) < 4 {
			continue
		}
		// Detect by magic
		if out[0] == 'B' && out[1] == 'L' && out[2] == 'P' { // BLP1/BLP2
			list.LoadUnknownTextures([]uint32{id})
			added++
			continue
		}
		if (out[0] == 'M' && out[1] == 'D' && out[2] == '2' && (out[3] == '0' || out[3] == '1')) || // M2
			(out[0] == 'P' && out[1] == 'W' && out[2] == 'M' && out[3] == 'O') { // WMO (store as unknown .m2 for now to match model ingestion path)
			list.LoadUnknownModels([]uint32{id})
			added++
			continue
		}
	}
	if added > 0 {
		log.Printf("unknown-ingest: added %d entries", added)
	}
}

// IngestUnknownsOnce runs unknown ingestion once in the background (exported wrapper).
func IngestUnknownsOnce(state cascState, list Listfile, limit int) {
	ingestUnknowns(state, list, limit)
}
