package config

import "testing"

func TestNormalizeOutputDirectory(t *testing.T) {
    cfg := &Config{Version: "2.0", Output: OutputConfig{Directory: "./custom-site///"}}
    res, err := NormalizeConfig(cfg)
    if err != nil { t.Fatalf("NormalizeConfig error: %v", err) }
    if cfg.Output.Directory != "custom-site" { t.Fatalf("expected cleaned directory 'custom-site', got %s", cfg.Output.Directory) }
    if len(res.Warnings) == 0 { t.Fatalf("expected warning for directory change") }
}