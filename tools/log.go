package tools

import (
	"encoding/json"
	"fmt"
	"time"
)

// LogJson prints structured logs in JSON format.
//
// This function is lightweight and intended for non-fatal, operational logging.
// Each log entry includes:
//   - UTC timestamp (RFC3339Nano format)
//   - Event name
//   - Any additional context fields
//
// Example:
//
//	LogJson("pebble_flush_error", map[string]any{
//		"error": err.Error(),
//	})
//
// Output (formatted):
//
//	{"time":"2025-11-11T10:15:42.458Z","event":"pebble_flush_error","error":"database is locked"}
//
// Note:
// This function prints directly to stdout and should be used only for
// lightweight diagnostic output within EchoPost. It is not meant for
// high-volume application logging.
func LogJson(event string, fields map[string]any) {
	entry := map[string]any{
		"time":  time.Now().UTC().Format(time.RFC3339Nano),
		"event": event,
	}

	// Merge provided context fields into the log entry
	for k, v := range fields {
		entry[k] = v
	}

	// Marshal the entry to JSON
	data, _ := json.Marshal(entry)

	// Print to stdout (used for lightweight observability)
	fmt.Println(string(data))
}
