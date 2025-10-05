package config

import "testing"

func TestNormalizeMonitoring(t *testing.T) {
	cfg := &Config{Version: "2.0", Monitoring: &MonitoringConfig{Logging: MonitoringLogging{Level: "DeBuG", Format: "JsOn"}}}
	res, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatalf("NormalizeConfig error: %v", err)
	}
	if cfg.Monitoring.Logging.Level != LogLevelDebug {
		t.Fatalf("expected debug got %s", cfg.Monitoring.Logging.Level)
	}
	if cfg.Monitoring.Logging.Format != LogFormatJSON {
		t.Fatalf("expected json got %s", cfg.Monitoring.Logging.Format)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected warnings")
	}
}

func TestNormalizeMonitoringUnknowns(t *testing.T) {
	cfg := &Config{Version: "2.0", Monitoring: &MonitoringConfig{Logging: MonitoringLogging{Level: "verbose", Format: "pretty"}}}
	res, err := NormalizeConfig(cfg)
	if err != nil {
		t.Fatalf("NormalizeConfig error: %v", err)
	}
	if cfg.Monitoring.Logging.Level != LogLevelInfo {
		t.Fatalf("fallback level info expected, got %s", cfg.Monitoring.Logging.Level)
	}
	if cfg.Monitoring.Logging.Format != LogFormatText {
		t.Fatalf("fallback format text expected, got %s", cfg.Monitoring.Logging.Format)
	}
	if len(res.Warnings) < 2 {
		t.Fatalf("expected >=2 warnings, got %d", len(res.Warnings))
	}
}
