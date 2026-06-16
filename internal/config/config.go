package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BrandonYaniz/yllmlog/internal/system"
	"gopkg.in/yaml.v3"
)

// Config is the minimal file-backed configuration for yllmlog.
type Config struct {
	DataDir string       `yaml:"data_dir"`
	YLLMD   YLLMDConfig  `yaml:"yllmd"`
	Daemon  DaemonConfig `yaml:"daemon"`
	Safety  SafetyConfig `yaml:"safety"`
}

type YLLMDConfig struct {
	Socket  string `yaml:"socket"`
	Profile string `yaml:"profile"`
}

type DaemonConfig struct {
	Socket string `yaml:"socket"`
}

type SafetyConfig struct {
	AllowChatMutations                 bool `yaml:"allow_chat_mutations"`
	RequireConfirmationForRiskyChanges bool `yaml:"require_confirmation_for_risky_changes"`
}

// Load reads, defaults, and validates a YAML config file.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	cfg := Default()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Default returns a config populated with defaults for the current OS.
func Default() Config {
	paths, err := system.DefaultPaths(runtime.GOOS)
	if err != nil {
		paths = system.Paths{}
	}
	return DefaultForPaths(paths)
}

// DefaultForPaths returns a config populated with the provided platform paths.
func DefaultForPaths(paths system.Paths) Config {
	return Config{
		DataDir: paths.DataDir,
		YLLMD: YLLMDConfig{
			Socket:  paths.YLLMDSocket,
			Profile: paths.YLLMDProfile,
		},
		Daemon: DaemonConfig{
			Socket: paths.DaemonSocket,
		},
		Safety: SafetyConfig{
			AllowChatMutations:                 true,
			RequireConfirmationForRiskyChanges: true,
		},
	}
}

// Validate checks the config values that are required before runtime services start.
func (c Config) Validate() error {
	if err := requireAbsolutePath("data_dir", c.DataDir); err != nil {
		return err
	}
	if err := requireAbsolutePath("yllmd.socket", c.YLLMD.Socket); err != nil {
		return err
	}
	if err := requireAbsolutePath("daemon.socket", c.Daemon.Socket); err != nil {
		return err
	}
	if strings.TrimSpace(c.YLLMD.Profile) == "" {
		return errors.New("yllmd.profile is required")
	}
	return nil
}

func requireAbsolutePath(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be an absolute path", name)
	}
	return nil
}
