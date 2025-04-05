package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// dbOperation represents a function to be executed on the database
type dbOperation func(*IPDB) interface{}

// IPDB represents the IP address database
type IPDB struct {
	VMToIP map[string]string `json:"vm_to_ip"` // Maps VM ID to IP address
	path   string            `json:"-"`        // Path to the database file
	opChan chan dbOperation  `json:"-"`        // Channel for serializing operations
	done   chan struct{}     `json:"-"`        // Channel to signal shutdown
}

// NewIPDB creates a new IP database
func NewIPDB(dbPath string) (*IPDB, error) {
	// Create database directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	db := &IPDB{
		VMToIP: make(map[string]string),
		path:   dbPath,
		opChan: make(chan dbOperation),
		done:   make(chan struct{}),
	}

	// Load existing database if it exists
	if _, err := os.Stat(dbPath); err == nil {
		data, err := os.ReadFile(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read database: %v", err)
		}
		if err := json.Unmarshal(data, db); err != nil {
			return nil, fmt.Errorf("failed to parse database: %v", err)
		}
	}

	// Start the database operation handler
	go db.handleOperations()

	return db, nil
}

// save writes the database to disk
func (db *IPDB) save() error {
	data, err := json.MarshalIndent(db, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.path, data, 0644)
}

// handleOperations processes database operations sequentially
func (db *IPDB) handleOperations() {
	for {
		select {
		case op := <-db.opChan:
			// Execute the operation function with the database
			op(db)
		case <-db.done:
			return
		}
	}
}

// Close shuts down the database operation handler
func (db *IPDB) Close() {
	close(db.done)
}

// AssignIP assigns an IP address to a VM
func (db *IPDB) AssignIP(vmID, ip string) error {
	response := make(chan error)
	db.opChan <- func(db *IPDB) interface{} {
		db.VMToIP[vmID] = ip
		err := db.save()
		response <- err
		return nil
	}
	return <-response
}

// GetIP gets the IP address assigned to a VM
func (db *IPDB) GetIP(vmID string) (string, bool, error) {
	response := make(chan struct {
		ip     string
		exists bool
		err    error
	})
	db.opChan <- func(db *IPDB) interface{} {
		ip, exists := db.VMToIP[vmID]
		response <- struct {
			ip     string
			exists bool
			err    error
		}{ip, exists, nil}
		return nil
	}
	result := <-response
	return result.ip, result.exists, result.err
}

// RemoveVM removes a VM from the database
func (db *IPDB) RemoveVM(vmID string) error {
	response := make(chan error)
	db.opChan <- func(db *IPDB) interface{} {
		delete(db.VMToIP, vmID)
		err := db.save()
		response <- err
		return nil
	}
	return <-response
}

// GetAssignedIPs returns a map of all assigned IPs
func (db *IPDB) GetAssignedIPs() (map[string]bool, error) {
	response := make(chan struct {
		ips map[string]bool
		err error
	})
	db.opChan <- func(db *IPDB) interface{} {
		ips := make(map[string]bool)
		for _, ip := range db.VMToIP {
			ips[ip] = true
		}
		response <- struct {
			ips map[string]bool
			err error
		}{ips, nil}
		return nil
	}
	result := <-response
	return result.ips, result.err
}

// Cleanup removes entries for VMs that no longer exist
func (db *IPDB) Cleanup(existingVMs map[string]bool) error {
	response := make(chan error)
	db.opChan <- func(db *IPDB) interface{} {
		for vmID := range db.VMToIP {
			if !existingVMs[vmID] {
				delete(db.VMToIP, vmID)
			}
		}
		err := db.save()
		response <- err
		return nil
	}
	return <-response
}
