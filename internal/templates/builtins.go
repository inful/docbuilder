package templates

import (
	"maps"
	"time"
)

func withBuiltinTemplateData(data map[string]any) map[string]any {
	needDate := data == nil
	needDateTime := data == nil
	if data != nil {
		if _, ok := data["Date"]; !ok {
			needDate = true
		}
		if _, ok := data["DateTime"]; !ok {
			needDateTime = true
		}
	}

	if !needDate && !needDateTime {
		return data
	}

	out := make(map[string]any, len(data)+2)
	maps.Copy(out, data)

	now := time.Now().UTC()
	if needDate {
		out["Date"] = now.Format("2006-01-02")
	}
	if needDateTime {
		out["DateTime"] = now.Format(time.RFC3339)
	}

	return out
}
