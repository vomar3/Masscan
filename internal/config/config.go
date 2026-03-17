package config

import (
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Masscan struct {
		Path string `yaml:"path"`
	} `yaml:"masscan"`

	Scanner struct {
		Rate    int      `yaml:"rate"`
		Ports   string   `yaml:"ports"`
		Targets []string `yaml:"targets"`
	} `yaml:"scanner"`

	Workers struct {
		Count     int `yaml:"count"`
		QueueSize int `yaml:"queue_size"`
	} `yaml:"workers"`

	Telegram struct {
		Enabled bool   `yaml:"enabled"`
		ChatID  string `yaml:"chat_id"`
	} `yaml:"telegram"`

	Email struct {
		Enabled bool   `yaml:"enabled"`
		To      string `yaml:"to"`
	} `yaml:"email"`

	Storage struct {
		DBPath string `yaml:"db_path"`
		File   string `yaml:"file"`
	} `yaml:"storage"`

	TelegramToken   string
	MailtrapToken   string
	MailtrapInboxID string
	EmailFrom       string
}

func LoadConfig(path string) (*Config, error) {
	_ = godotenv.Load()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = "results.db"
	}
	if cfg.Storage.File == "" {
		cfg.Storage.File = "results.json"
	}

	cfg.TelegramToken = os.Getenv("TELEGRAM_TOKEN")
	cfg.MailtrapToken = os.Getenv("MAILTRAP_API_TOKEN")
	cfg.MailtrapInboxID = os.Getenv("MAILTRAP_INBOX_ID")
	cfg.EmailFrom = os.Getenv("EMAIL_FROM")

	return &cfg, nil
}
