package models

type ScanResult struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Service string `json:"service"`
	Banner  string `json:"banner"`
}
