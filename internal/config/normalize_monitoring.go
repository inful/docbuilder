package config

// no extra imports required

func normalizeMonitoring(m **MonitoringConfig, res *NormalizationResult) {
    if m == nil || *m == nil { return }
    cfg := *m
    if lvl := NormalizeLogLevel(string(cfg.Logging.Level)); lvl != "" {
        if cfg.Logging.Level != lvl {
            res.Warnings = append(res.Warnings, warnChanged("monitoring.logging.level", cfg.Logging.Level, lvl))
            cfg.Logging.Level = lvl
        }
    } else if string(cfg.Logging.Level) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("monitoring.logging.level", string(cfg.Logging.Level), string(LogLevelInfo)))
        cfg.Logging.Level = LogLevelInfo
    }
    if f := NormalizeLogFormat(string(cfg.Logging.Format)); f != "" {
        if cfg.Logging.Format != f {
            res.Warnings = append(res.Warnings, warnChanged("monitoring.logging.format", cfg.Logging.Format, f))
            cfg.Logging.Format = f
        }
    } else if string(cfg.Logging.Format) != "" {
        res.Warnings = append(res.Warnings, warnUnknown("monitoring.logging.format", string(cfg.Logging.Format), string(LogFormatText)))
        cfg.Logging.Format = LogFormatText
    }
}
