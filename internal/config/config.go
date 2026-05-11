package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	userScope "github.com/MaxRomanov007/smart-pc-go-lib/user-scope"
	"github.com/ilyakaznacheev/cleanenv"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Env        string     `yaml:"env"         env-default:"production"`
	LogPath    string     `yaml:"log_path"                             env:"LOG_PATH"`
	HTTPServer HTTPServer `yaml:"http_server"`
	MQTT       MQTT       `yaml:"mqtt"`
	Auth       Auth       `yaml:"auth"`
	Services   Services   `yaml:"services"`
	Checker    Checker    `yaml:"checker"`
	Storage    Storage    `yaml:"storage"`

	file *os.File `yaml:"-"`
}

type Storage struct {
	Pcs       []Pc          `yaml:"pcs"`
	AuthToken *oauth2.Token `yaml:"auth_token"`
}

type HTTPServer struct {
	Address         string        `yaml:"address"          env-default:"0.0.0.0:8506"`
	Timeout         time.Duration `yaml:"timeout"          env-default:"4s"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"     env-default:"60s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env-default:"1s"`
}

type Pc struct {
	ID  string `yaml:"id"`
	MAC string `yaml:"mac"`
}

type MQTT struct {
	BrokerURL             string `yaml:"broker_url"              env-default:"wss://mqtt.smartpc.site/mqtt"`
	ClientIDPrefix        string `yaml:"client_id_prefix"        env-default:"smart_pc_waker_"`
	SessionExpiryInterval uint32 `yaml:"session_expiry_interval" env-default:"60"`
	KeepAlive             uint16 `yaml:"keep_alive"              env-default:"20"`
}

type Auth struct {
	Oauth2      Oauth2 `yaml:"oauth2"`
	CallbackURL string `yaml:"callback_url" env-default:"http://127.0.0.1:8506/waker/auth/callback"`
	UserinfoURL string `yaml:"userinfo_url" env-default:"https://hydra.smartpc.site/userinfo"`
}

type Oauth2 struct {
	ClientID string         `yaml:"client_id" env-default:"smart-pc-waker"`
	Scopes   []string       `yaml:"scopes"    env-default:"offline,mqtt:pc:state:write,mqtt:pc:command:read,mqtt:pc:log:write,mqtt:pc:status:write"`
	Endpoint Oauth2Endpoint `yaml:"endpoint"`
}

type Oauth2Endpoint struct {
	AuthURL  string `yaml:"auth_url"  env-default:"https://hydra.smartpc.site/oauth2/auth"`
	TokenURL string `yaml:"token_url" env-default:"https://hydra.smartpc.site/oauth2/token"`
}

type Services struct {
	Pcs PcsService `yaml:"pcs"`
}

type PcsService struct {
	Timeout time.Duration `yaml:"timeout"  env-default:"5s"`
	BaseURL string        `yaml:"base_url" env-default:"https://api.smartpc.site/pcs"`
}

type Checker struct {
	Interval time.Duration `yaml:"interval" env-default:"24h"`
}

func MustLoad(ctx context.Context) *Config {
	cfg := new(Config)

	if err := cfg.openConfigFile(); err != nil {
		panic(err)
	}

	isNew, err := cfg.isEmptyFile()
	if err != nil {
		panic(err)
	}

	if err := cfg.readAndApplyDefaults(); err != nil {
		panic(err)
	}
	if err := cfg.applyLogPathDefault(); err != nil {
		panic(err)
	}

	if isNew { // ← fix #3: сохраняем только если файл был новым
		if err := cfg.Save(); err != nil {
			panic(err)
		}
	}

	go func() {
		<-ctx.Done()
		_ = cfg.file.Close()
	}()

	return cfg
}

// readAndApplyDefaults читает файл через cleanenv (yaml + env-default теги).
// cleanenv.ReadConfig требует путь к файлу, поэтому передаём его имя,
// а не читаем из уже открытого fd — файл к этому моменту уже существует.
func (c *Config) readAndApplyDefaults() error {
	const op = "config.readAndApplyDefaults"

	info, err := c.file.Stat()
	if err != nil {
		return fmt.Errorf("%s: failed to stat file: %w", op, err)
	}

	if info.Size() == 0 {
		// Пустой файл: применяем дефолты из struct-тегов и переменных окружения
		// cleanenv.ReadEnv не парсит YAML, поэтому не упадёт на пустом входе
		if err := cleanenv.ReadEnv(c); err != nil {
			return fmt.Errorf("%s: failed to set defaults: %w", op, err)
		}
		return nil
	}

	// Если файл пустой — cleanenv всё равно применит env-default значения.
	// Если непустой — распарсит yaml и поверх наложит env-переменные.
	if err := cleanenv.ReadConfig(c.file.Name(), c); err != nil {
		return fmt.Errorf("%s: failed to read config file: %w", op, err)
	}

	return nil
}

// applyLogPathDefault выставляет LogPath если он не задан в файле/env.
func (c *Config) applyLogPathDefault() error {
	const op = "config.applyLogPathDefault"

	if c.LogPath != "" {
		return nil
	}

	lp, err := userScope.NewCachePath("log.log")
	if err != nil {
		return fmt.Errorf("%s: failed to create log path: %w", op, err)
	}
	c.LogPath = string(lp)

	return nil
}

func (c *Config) openConfigFile() error {
	const op = "config.openConfigFile"

	cp := os.Getenv("CONFIG_PATH")
	if cp == "" {
		path, err := userScope.NewCachePath("config.yaml")
		if err != nil {
			return fmt.Errorf("%s: failed to create user scoped path: %w", op, err)
		}
		cp = string(path)
	}

	if err := os.MkdirAll(filepath.Dir(cp), 0o755); err != nil {
		return fmt.Errorf("%s: cannot create directories: %w", op, err)
	}

	f, err := os.OpenFile(cp, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %w", op, err)
	}
	c.file = f

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
		return fmt.Errorf("%s: %w", op, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("%s: failed to marshal config: %w", op, err)
	}

	if _, err := c.file.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("%s: failed to write file: %w", op, err)
	}

	if err := c.file.Sync(); err != nil {
		return fmt.Errorf("%s: failed to sync file: %w", op, err)
	}

	return nil
}

func (c *Config) isEmptyFile() (bool, error) {
	info, err := c.file.Stat()
	if err != nil {
		return false, fmt.Errorf("config.isEmptyFile: %w", err)
	}
	return info.Size() == 0, nil
}
