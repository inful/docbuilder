package plugin

import (
	"context"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// PluginContext provides plugins with access to DocBuilder services and state.
// This context is passed to plugins during execution, allowing them to interact
// with the build system without tight coupling.
type PluginContext struct {
	// Context is the standard Go context for cancellation and deadlines.
	Context context.Context

	// Logger provides structured logging for plugin operations.
	Logger *slog.Logger

	// Config is the DocBuilder configuration.
	Config *config.Config

	// WorkspaceDir is the temporary directory for this build.
	WorkspaceDir string

	// OutputDir is where the Hugo site should be generated.
	OutputDir string

	// BuildID uniquely identifies this build.
	BuildID string

	// Data is a map for plugins to share data during execution.
	// This allows plugins to communicate state without direct dependencies.
	Data map[string]interface{}
}

// NewPluginContext creates a new plugin context with the given services.
func NewPluginContext(
	ctx context.Context,
	logger *slog.Logger,
	cfg *config.Config,
	workspaceDir, outputDir, buildID string,
) *PluginContext {
	return &PluginContext{
		Context:      ctx,
		Logger:       logger,
		Config:       cfg,
		WorkspaceDir: workspaceDir,
		OutputDir:    outputDir,
		BuildID:      buildID,
		Data:         make(map[string]interface{}),
	}
}

// WithValue returns a copy of the context with the given key-value pair in Data.
func (pc *PluginContext) WithValue(key string, value interface{}) *PluginContext {
	newData := make(map[string]interface{}, len(pc.Data)+1)
	for k, v := range pc.Data {
		newData[k] = v
	}
	newData[key] = value

	return &PluginContext{
		Context:      pc.Context,
		Logger:       pc.Logger,
		Config:       pc.Config,
		WorkspaceDir: pc.WorkspaceDir,
		OutputDir:    pc.OutputDir,
		BuildID:      pc.BuildID,
		Data:         newData,
	}
}

// GetValue retrieves a value from the plugin data map.
// Returns nil if the key doesn't exist.
func (pc *PluginContext) GetValue(key string) interface{} {
	return pc.Data[key]
}

// GetString retrieves a string value from the plugin data map.
// Returns empty string if the key doesn't exist or is not a string.
func (pc *PluginContext) GetString(key string) string {
	if v, ok := pc.Data[key].(string); ok {
		return v
	}
	return ""
}

// GetBool retrieves a boolean value from the plugin data map.
// Returns false if the key doesn't exist or is not a boolean.
func (pc *PluginContext) GetBool(key string) bool {
	if v, ok := pc.Data[key].(bool); ok {
		return v
	}
	return false
}

// GetInt retrieves an integer value from the plugin data map.
// Returns 0 if the key doesn't exist or is not an integer.
func (pc *PluginContext) GetInt(key string) int {
	if v, ok := pc.Data[key].(int); ok {
		return v
	}
	return 0
}

// LogInfo logs an informational message with plugin context.
func (pc *PluginContext) LogInfo(msg string, args ...interface{}) {
	pc.Logger.Info(msg, args...)
}

// LogWarn logs a warning message with plugin context.
func (pc *PluginContext) LogWarn(msg string, args ...interface{}) {
	pc.Logger.Warn(msg, args...)
}

// LogError logs an error message with plugin context.
func (pc *PluginContext) LogError(msg string, args ...interface{}) {
	pc.Logger.Error(msg, args...)
}

// LogDebug logs a debug message with plugin context.
func (pc *PluginContext) LogDebug(msg string, args ...interface{}) {
	pc.Logger.Debug(msg, args...)
}
