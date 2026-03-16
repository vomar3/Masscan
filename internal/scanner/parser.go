package scanner

import (
	"encoding/json"
	"os"
)

type MasscanResult struct {
	IP string `json:"ip"`

	Ports []struct {
		Port int `json:"port"`
	} `json:"ports"`
}

func ParseResults(file string) ([]MasscanResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var results []MasscanResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return results, nil
}
