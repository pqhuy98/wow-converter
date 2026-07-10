package testcases

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RandomM2Case is a local M2/WMO export parity case with optional skin override.
type RandomM2Case struct {
	OutputName string `json:"outputName"`
	LocalRef   string `json:"localRef"`
	SkinID     string `json:"skinId"`
	FileDataID int    `json:"fileDataID"`
	FileName   string `json:"fileName"`
	ModelType  string `json:"modelType"`
}

// RandomM2CasesFile is the JSON payload written by scripts/mdl-parity-random-cases.ts.
type RandomM2CasesFile struct {
	Seed  int            `json:"seed"`
	Count int            `json:"count"`
	Cases []RandomM2Case `json:"cases"`
}

// LoadRandomM2Cases reads random M2/WMO parity cases from a JSON file.
func LoadRandomM2Cases(path string) (RandomM2CasesFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RandomM2CasesFile{}, err
	}
	var payload RandomM2CasesFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return RandomM2CasesFile{}, err
	}
	for i := range payload.Cases {
		if payload.Cases[i].ModelType == "" {
			if strings.HasSuffix(strings.ToLower(payload.Cases[i].FileName), ".wmo") {
				payload.Cases[i].ModelType = "wmo"
			} else {
				payload.Cases[i].ModelType = "m2"
			}
		}
	}
	if len(payload.Cases) == 0 {
		return RandomM2CasesFile{}, fmt.Errorf("random cases file %s has no cases", path)
	}
	return payload, nil
}
