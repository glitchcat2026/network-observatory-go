package internal

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ConfigFile = "collector-config.json"

const (
	IPVersionIPv4 = "ipv4"
	IPVersionIPv6 = "ipv6"
)

func NormalizeIPVersion(version string) (string, error) {
	if version != IPVersionIPv4 && version != IPVersionIPv6 {
		return "", fmt.Errorf("ip_version must be %q or %q", IPVersionIPv4, IPVersionIPv6)
	}
	return version, nil
}

func NormalizeLanguage(language string) (string, error) {
	language = strings.ToLower(language)
	if _, ok := Messages[language]; !ok {
		return "", fmt.Errorf("unsupported lang %q; add it to internal/i18n.go", language)
	}
	return language, nil
}

func ExecutableRelativePath(name string) (string, error) {
	if name == ":memory:" || filepath.IsAbs(name) {
		return name, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), name), nil
}

type Config struct {
	DBPath      string `json:"db_path"`
	Addr        string `json:"addr"`
	Token       string `json:"token"`
	MaxBodySize int64  `json:"max_body_size"`
	Lang        string `json:"lang"`
}

func GenerateToken() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func DefaultConfig() Config {
	return Config{
		DBPath:      "observatory.db",
		Addr:        ":8080",
		Token:       GenerateToken(),
		MaxBodySize: 1 << 20,
		Lang:        "en",
	}
}

func LoadOrCreateConfig(path string) (Config, bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		data, err := json.MarshalIndent(
			cfg,
			"",
			"    ",
		)
		if err != nil {
			return cfg, false, err
		}
		err = os.WriteFile(
			path,
			data,
			0644,
		)
		if err != nil {
			return cfg, false, err
		}
		fmt.Println(
			"collector-config.json created. Please edit it and restart.",
		)
		return cfg, true, nil
	}

	if err != nil {
		return Config{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, false, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values are not allowed")
		}
		return cfg, false, err
	}
	cfg.Lang, err = NormalizeLanguage(cfg.Lang)
	if err != nil {
		return cfg, false, err
	}
	return cfg, false, nil
}
