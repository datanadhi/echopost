package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	pb "github.com/datanadhi/echopost/logagentpb"

	"github.com/cockroachdb/pebble"
	flow "github.com/datanadhi/flowhttp/client"
)

// logRecord represents the structure of each log stored in Pebble.
// It holds the payload (actual log data), pipeline identifiers, and timestamp.
type logRecord struct {
	Payload    map[string]any `json:"payload"`
	Pipelines  []string       `json:"pipelines"`
	ReceivedAt string         `json:"received_at"`
}

// SendLog handles gRPC log requests coming from the SDK or application.
// It stores incoming logs into Pebble with a unique key, ensuring persistence
// even if the main server is unreachable.
func (s *server) SendLog(ctx context.Context, req *pb.LogRequest) (*pb.LogResponse, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(req.JsonData), &out); err != nil {
		out = map[string]any{}
	}
	rec := logRecord{
		Payload:    out,
		Pipelines:  req.Pipelines,
		ReceivedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	data, _ := json.Marshal(rec)
	key := fmt.Sprintf("%d_%d", time.Now().UnixNano(), rand.Intn(1000))

	if err := s.config.Db.Set([]byte(key), data, pebble.NoSync); err != nil {
		LogJson("pebble_write_error", map[string]any{"error": err.Error()})
		return &pb.LogResponse{Success: false, Message: "Db write failed"}, nil
	}

	LogJson("log_stored", map[string]any{"key": key})
	return &pb.LogResponse{Success: true, Message: "stored"}, nil
}

// FlushPebbleDB ensures that all in-memory data is written to disk
// and the write-ahead log (WAL) is synced. This prevents data loss
// if the agent crashes or is terminated unexpectedly.
func FlushPebbleDB(db *pebble.DB) {
	if db != nil {
		if err := db.Flush(); err != nil {
			LogJson("pebble_flush_error", map[string]any{"error": err.Error()})
		}
		if err := db.LogData(nil, pebble.Sync); err != nil {
			LogJson("pebble_wal_sync_error", map[string]any{"error": err.Error()})
		}
	}
}

// FlushPebbleDBOnInterval runs a background goroutine that periodically flushes
// Pebble to disk every second. It stops automatically when the context is canceled.
func (c *ServerConfig) FlushPebbleDBOnInterval(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				LogJson("flusher_stopping", nil)
				return
			case <-ticker.C:
				FlushPebbleDB(c.Db)
			}
		}
	}()
}

// deleteKeysBatch removes a batch of keys from Pebble in a single atomic operation.
// It uses a write batch for better efficiency and durability.
func deleteKeysBatch(db *pebble.DB, keys [][]byte) error {
	batch := db.NewBatch()
	defer batch.Close()

	for _, key := range keys {
		if err := batch.Delete(key, nil); err != nil {
			return err
		}
	}

	return batch.Commit(pebble.NoSync)
}

// ProcessPebble scans through all stored logs in Pebble and sends them to the main server.
// It removes logs that were successfully delivered or permanently failed (4xx/5xx <= 500),
// while retaining those that failed due to transient errors (5xx > 500).
func (c *ServerConfig) ProcessPebble(ctx context.Context) error {
	client := flow.NewClient(5 * time.Second)
	var serverErr error

	FlushPebbleDB(c.Db)
	defer FlushPebbleDB(c.Db)

	iter, err := c.Db.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	var keys [][]byte
	count := 0

	for iter.First(); iter.Valid(); iter.Next() {
		// Stop processing if context canceled
		if ctx.Err() != nil {
			break
		}

		var rec logRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			LogJson("pebble_read_error", map[string]any{"error": err.Error()})
			continue
		}

		addKey, err := c.sendToServer(rec, client)
		if err != nil {
			serverErr = err
			break
		}

		if addKey {
			count++
			keyCopy := make([]byte, len(iter.Key()))
			copy(keyCopy, iter.Key())
			keys = append(keys, keyCopy)
		}
	}

	// Delete successfully processed or permanently failed records
	if len(keys) > 0 {
		if err := deleteKeysBatch(c.Db, keys); err != nil {
			LogJson("pebble_delete_error", map[string]any{"error": err.Error()})
			return err
		}
		LogJson("pebble_processed", map[string]any{"processed_count": count})
	} else {
		LogJson("pebble_processed_none", nil)
	}

	return serverErr
}

// PebbleIsEmpty checks if the Pebble database is empty.
// Used mainly during agent shutdown to decide whether to delete the DB directory.
func PebbleIsEmpty(db *pebble.DB) bool {
	if db == nil {
		return true
	}

	iter, err := db.NewIter(nil)
	if err != nil {
		return true
	}
	defer iter.Close()

	return !iter.First()
}
