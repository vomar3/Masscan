package storage

import (
	"encoding/json"
	"os"

	"masscan/internal/models"
)

func Load(file string) ([]models.ScanResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.ScanResult{}, nil
		}
		return nil, err
	}

	var results []models.ScanResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func Save(file string, data []models.ScanResult) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(file, jsonData, 0644)
}
