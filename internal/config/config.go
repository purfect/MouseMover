package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Enabled                 bool   `json:"enabled"`
	IdleTimeoutSeconds      int    `json:"idleTimeoutSeconds"`
	MovementIntervalSeconds int    `json:"movementIntervalSeconds"`
	MovementPixels          int    `json:"movementPixels"`
	Mode                    string `json:"mode"`
	KeepDisplayAwake        bool   `json:"keepDisplayAwake"`
	StartWithWindows        bool   `json:"startWithWindows"`
	AutoActivate            bool   `json:"autoActivate"`
	MinimizeOnActivate      bool   `json:"minimizeOnActivate"`
	StartMinimized          bool   `json:"startMinimized"`
	NotificationsEnabled    bool   `json:"notificationsEnabled"`
}

func Default() Config {
	return Config{IdleTimeoutSeconds: 60, MovementIntervalSeconds: 30, MovementPixels: 1, Mode: "mouse", MinimizeOnActivate: true, NotificationsEnabled: true}
}

func (c Config) Validate() error {
	if c.IdleTimeoutSeconds < 1 || c.IdleTimeoutSeconds > 86400 {
		return errors.New("Inaktivitätszeit muss zwischen 1 und 86400 Sekunden liegen")
	}
	if c.MovementIntervalSeconds < 1 || c.MovementIntervalSeconds > 86400 {
		return errors.New("Intervall muss zwischen 1 und 86400 Sekunden liegen")
	}
	if c.MovementPixels < 1 || c.MovementPixels > 20 {
		return errors.New("Bewegungsdistanz muss zwischen 1 und 20 Pixeln liegen")
	}
	if c.Mode != "mouse" && c.Mode != "awake" && c.Mode != "combined" {
		return errors.New("Ungültiger Betriebsmodus")
	}
	return nil
}

func Path() (string, error) {
	p := os.Getenv("APPDATA")
	if p == "" {
		return "", errors.New("APPDATA ist nicht gesetzt")
	}
	return filepath.Join(p, "Mousemover", "config.json"), nil
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		c := Default()
		return c, Save(path, c)
	}
	if err != nil {
		return Default(), err
	}
	var c Config
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if err = json.Unmarshal(b, &c); err != nil {
		return Default(), fmt.Errorf("Konfiguration beschädigt: %w", err)
	}
	if err = c.Validate(); err != nil {
		return Default(), fmt.Errorf("Konfiguration ungültig: %w", err)
	}
	return c, nil
}

func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
