package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	userScope "github.com/MaxRomanov007/smart-pc-go-lib/user-scope"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Env     string               `yaml:"env"`
	Pcs     []Pc                 `yaml:"pcs"`
	LogPath *userScope.CachePath `yaml:"log_path"`

	file *os.File
}

type Pc struct {
	ID  uuid.UUID `yaml:"id"`
	MAC string    `yaml:"mac"`
}

func MustLoad(ctx context.Context) *Config {
	cfg := new(Config)

	if err := cfg.openConfigFile(); err != nil {
		panic(err)
	}
	if err := cfg.readFile(); err != nil {
		panic(err)
	}
	if err := cfg.setDefaults(); err != nil {
		panic(err)
	}
	if err := cfg.Save(); err != nil {
		panic(err)
	}

	go func() {
		<-ctx.Done()
		_ = cfg.file.Close()
	}()

	return cfg
}

func (c *Config) openConfigFile() error {
	const op = "config.configPath"

	cp := os.Getenv("CONFIG_PATH")

	if cp == "" {
		path, err := userScope.NewCachePath("config.yaml")
		if err != nil {
			return fmt.Errorf("%s: failed to create user scoped path: %w", op, err)
		}
		cp = string(path)
	}

	if err := os.MkdirAll(filepath.Dir(cp), 0o755); err != nil {
		panic(fmt.Errorf("cannot create directories: %w", err))
	}
	f, err := os.OpenFile(cp, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %w", op, err)
	}
	c.file = f

	return nil
}

func (c *Config) setDefaults() error {
	const op = "config.setDefaults"

	if c.Env == "" {
		c.Env = "production"
	}
	if c.Pcs == nil {
		c.Pcs = []Pc{}
	}
	logPath, err := userScope.NewCachePath("log.log")
	if err != nil {
		return fmt.Errorf("%s: failed to create log path: %w", op, err)
	}
	c.LogPath = &logPath

	return nil
}

func (c *Config) readFile() error {
	const op = "config.readFile"

	buf := bytes.NewBuffer(nil)
	if _, err := buf.ReadFrom(c.file); err != nil {
		return fmt.Errorf("%s: failed to read file: %w", op, err)
	}

	if err := yaml.Unmarshal(buf.Bytes(), c); err != nil {
		return fmt.Errorf("%s: failed to unmarshal file: %w", op, err)
	}

	return nil
}

func (c *Config) clearFile() error {
	const op = "config.clearFile"

	if err := c.file.Truncate(0); err != nil {
		return fmt.Errorf("%s: failed to truncate file: %w", op, err)
	}
	if _, err := c.file.Seek(0, 0); err != nil {
		return fmt.Errorf("%s: failed to seek file: %w", op, err)
	}

	return nil
}

func (c *Config) Save() error {
	const op = "config.Save"

	if err := c.clearFile(); err != nil {
		return fmt.Errorf("%s: failed to clear file: %w", op, err)
	}

	f, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("%s: failed to marshal file: %w", op, err)
	}

	if _, err := c.file.Write(f); err != nil {
		return fmt.Errorf("%s: failed to write file: %w", op, err)
	}

	return nil
}
