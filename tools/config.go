package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
)

// Files holds all the file handles and paths used by the agent.
// This includes session logs, the socket path, and the "accepting" flag file.
type Files struct {
	acceptingFlagPath string   // Internal path for the accepting flag
	AcceptingFlag     *os.File // File handle for the accepting flag
	successLog        *os.File // File handle for successful log writes
	failureLog        *os.File // File handle for failed log writes
	SocketPath        string   // Path for the agent's Unix socket
	dbPath            string   // Path for the Pebble DB directory
}

// ServerConfig contains runtime configuration and references for the running agent.
// It holds API credentials, the target server host, and file/database handles.
type ServerConfig struct {
	ApiKey     string     // API key used for authenticating with the main server
	ServerHost string     // Base URL of the main Data Nadhi server
	Db         *pebble.DB // Local Pebble database instance
	Files                 // Embedded struct for managing all file paths and handles
}

// CreateRequiredFiles sets up the local file structure required for the agent session.
// It creates session folders, log files, the socket path, and opens Pebble DB.
func (c *ServerConfig) CreateRequiredFiles(baseDir string) error {
	var err error

	// Create session folder with timestamped name
	sessionPath := filepath.Join(
		baseDir,
		fmt.Sprintf("session-%s", time.Now().UTC().Format("2006-01-02T15-04-05Z")),
	)
	if err = os.MkdirAll(sessionPath, 0755); err != nil {
		return err
	}

	// Prepare paths for control flag and logs
	c.acceptingFlagPath = filepath.Join(baseDir, "agent-status.lock")

	successLogPath := filepath.Join(sessionPath, "agent-success.log")
	c.successLog, err = os.OpenFile(successLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	failureLogPath := filepath.Join(sessionPath, "agent-failure.log")
	c.failureLog, err = os.OpenFile(failureLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Prepare Unix socket and Pebble DB directories
	c.SocketPath = filepath.Join(baseDir, "data-nadhi-agent.sock")
	c.dbPath = filepath.Join(baseDir, "pebble")

	// Initialize Pebble database
	c.Db, err = pebble.Open(c.dbPath, &pebble.Options{})
	return err
}

// CloseFiles safely closes all open file handles and cleans up temporary artifacts.
// Removes the Unix socket and deletes the Pebble directory if it's empty.
func (c *ServerConfig) CloseFiles() {
	files := []*os.File{c.AcceptingFlag, c.successLog, c.failureLog}

	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}

	// Remove the Unix socket file
	_ = os.Remove(c.SocketPath)

	// Close Pebble DB and delete it if it's empty
	shouldRemovePebble := PebbleIsEmpty(c.Db)
	if c.Db != nil {
		_ = c.Db.Close()
		if shouldRemovePebble {
			_ = os.RemoveAll(c.dbPath)
		}
	}
}

// EnableAcceptingFlag creates the lock file to indicate the agent is accepting logs.
// Returns an error if the file already exists (agent already accepting).
func (c *ServerConfig) EnableAcceptingFlag() error {
	path := c.acceptingFlagPath
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	c.AcceptingFlag = f
	return nil
}

// DisableAcceptingFlag removes the accepting flag file and closes its handle.
// This signals that the agent has stopped accepting new logs.
func (c *ServerConfig) DisableAcceptingFlag() error {
	if c.AcceptingFlag != nil {
		_ = c.AcceptingFlag.Close()
		c.AcceptingFlag = nil
	}
	_ = os.Remove(c.acceptingFlagPath)
	return nil
}
