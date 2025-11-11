package tools

import (
	"bytes"
	"encoding/json"
	"fmt"

	flow "github.com/datanadhi/flowhttp/client"
)

// logToFile writes the given record to either the success or failure log file.
// extras can contain any additional context like response payload or status codes.
func (c *ServerConfig) logToFile(rec logRecord, isSuccess bool, extras map[string]any) {
	logRecord := map[string]any{
		"log_data": rec,
		"context":  extras,
	}

	data, _ := json.Marshal(logRecord)

	if isSuccess && c.successLog != nil {
		_, _ = c.successLog.Write(append(data, '\n'))
	} else if !isSuccess && c.failureLog != nil {
		_, _ = c.failureLog.Write(append(data, '\n'))
	}
}

// sendToServer pushes a single log record to the Data Nadhi server.
// It returns true if the record should be deleted from Pebble after sending,
// or false if it should be retried later.
//
// Rules:
// - 2xx  → success, remove from Pebble
// - 3xx–5xx (≤500) → permanent failure, log and remove from Pebble
// - >500 → transient server error, keep in Pebble for retry
func (c *ServerConfig) sendToServer(rec logRecord, client *flow.Client) (bool, error) {
	triggerURL := fmt.Sprintf("%s/log", c.ServerHost)

	// Prepare request body
	payload := map[string]any{
		"pipelines": rec.Pipelines,
		"log_data":  rec.Payload,
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		LogJson("json_marshal_error", map[string]any{"error": err.Error()})
		return false, nil
	}

	// Send request
	headers := map[string]string{"DATANADHI-API-KEY": c.ApiKey}
	resp, err := client.Post(triggerURL, nil, headers, bytes.NewBuffer(jsonBody), "application/json")
	if err != nil {
		LogJson("trigger_post_error", map[string]any{"error": err.Error()})
		return false, err
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	// Successful response — mark record as delivered
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.logToFile(rec, true, map[string]any{})
		return true, nil
	}

	// Non-retryable error (e.g. 401, 404, 422, etc.)
	if resp.StatusCode >= 300 && resp.StatusCode <= 500 {
		respString, _ := resp.String()
		c.logToFile(rec, false, map[string]any{
			"response":     respString,
			"responseCode": resp.StatusCode,
		})
		LogJson("trigger_client_error_final", map[string]any{"status": resp.StatusCode})
		return true, nil
	}

	// Transient server error (e.g. 502, 503, 504)
	if resp.StatusCode > 500 {
		LogJson("trigger_server_error", map[string]any{"status": resp.StatusCode})
		return false, fmt.Errorf("server_error, status %d", resp.StatusCode)
	}

	// Fallback (should not happen, just avoid retry loop)
	return true, nil
}

// IsHealthSuccess performs a simple health check on the main server.
// Returns true if the server responds with HTTP 200.
func (c *ServerConfig) IsHealthSuccess(client *flow.Client) bool {
	req, err := client.Get(c.ServerHost, nil, nil)
	if err != nil {
		LogJson("health_check_error", map[string]any{"error": err.Error()})
		return false
	}
	return req.StatusCode == 200
}
