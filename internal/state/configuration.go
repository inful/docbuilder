package state

import (
	"context"
)

// SetLastConfigHash stores the last config hash.
func (a *ServiceAdapter) SetLastConfigHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_config_hash", hash)
}

// GetLastConfigHash returns the last config hash.
func (a *ServiceAdapter) GetLastConfigHash() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_config_hash")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}

// SetLastReportChecksum stores the last report checksum.
func (a *ServiceAdapter) SetLastReportChecksum(sum string) {
	if sum == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_report_checksum", sum)
}

// GetLastReportChecksum returns the last report checksum.
func (a *ServiceAdapter) GetLastReportChecksum() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_report_checksum")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}

// SetLastGlobalDocFilesHash stores the global doc files hash.
func (a *ServiceAdapter) SetLastGlobalDocFilesHash(hash string) {
	if hash == "" {
		return
	}
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	_ = store.Set(ctx, "last_global_doc_files_hash", hash)
}

// GetLastGlobalDocFilesHash returns the global doc files hash.
func (a *ServiceAdapter) GetLastGlobalDocFilesHash() string {
	ctx := context.Background()
	store := a.service.GetConfigurationStore()
	result := store.Get(ctx, "last_global_doc_files_hash")
	if result.IsErr() {
		return ""
	}
	opt := result.Unwrap()
	if opt.IsNone() {
		return ""
	}
	if s, ok := opt.Unwrap().(string); ok {
		return s
	}
	return ""
}
