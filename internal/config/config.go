package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Debug           bool   `json:"debug"`
	TelegramToken   string `json:"telegram_token"`
	OmdbAPIKey      string `json:"omdb_api_key"`
	LanguageDefault string `json:"language_fallback"`
	MaxAlternatives int    `json:"max_alternatives"`

	Storage StorageConfig `json:"storage"`
}

type StorageConfig struct {
	MoviesFile       string        `json:"movies_file"`
	MessageIndexFile string        `json:"message_index_file"`
	SessionTTL       time.Duration `json:"session_ttl"`
	MaxMessages      int 		   `json:"max_messages"`
}

// Load reads the config file. If it does not exist, it creates a template but
// returns an error to force user intervention.
func Load(configDir string) (*Config, error) {
	log.Printf("[CONFIG] Initializing configuration")
	log.Printf("[CONFIG] Config directory: %s", configDir)

	cfgPath := filepath.Join(configDir, "config.json")
	log.Printf("[CONFIG] Config file path: %s", cfgPath)

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Printf("[CONFIG][ERROR] Failed to create config directory: %v", err)
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		log.Printf("[CONFIG][ERROR] Config file does not exist. Writing template and exiting.")
		template := Config{
			Debug:           false,
			TelegramToken:   "PUT_TELEGRAM_TOKEN_HERE",
			OmdbAPIKey:      "PUT_OMDB_API_KEY_HERE",
			LanguageDefault: "en",
			MaxAlternatives: 5,
			Storage: StorageConfig{
				MoviesFile:       "/config/data/movies.json",
				MessageIndexFile: "/config/data/message_index.json",
				SessionTTL:       30 * time.Second,
				MaxMessages:      10,
			},
		}

		data, _ := json.MarshalIndent(template, "", "  ")
		_ = os.WriteFile(cfgPath, data, 0644)
		log.Printf("[CONFIG] Template written to %s", cfgPath)
		log.Printf("[CONFIG] Please edit the file with your real tokens and restart the bot")
		return nil, fmt.Errorf("config file not found: %s", cfgPath)
	}

	// Load existing config
	log.Printf("[CONFIG] Loading config file")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file: %w", err)
	}

	// Log loaded configuration
	log.Printf("[CONFIG] Configuration loaded successfully")
	log.Printf("[CONFIG] Debug: %v, Language: %s, MaxAlt: %d", cfg.Debug, cfg.LanguageDefault, cfg.MaxAlternatives)
	log.Printf("[CONFIG] Storage: Movies=%s, Index=%s, SessionTTL=%s, MaxMessages=%s",
		cfg.Storage.MoviesFile, cfg.Storage.MessageIndexFile, cfg.Storage.SessionTTL, cfg.Storage.MaxMessages)

	// Warn if tokens not set
	if cfg.TelegramToken == "" || cfg.TelegramToken == "PUT_TELEGRAM_TOKEN_HERE" {
		log.Printf("[CONFIG][WARN] Telegram token is not set")
	}
	if cfg.OmdbAPIKey == "" || cfg.OmdbAPIKey == "PUT_OMDB_API_KEY_HERE" {
		log.Printf("[CONFIG][WARN] OMDb API key is not set")
	}

	return &cfg, nil
}