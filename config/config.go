package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// VCenterConfig holds the vCenter specific configuration
type VCenterConfig struct {
	IP         string `json:"ip"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Datacenter string `json:"datacenter"`
	Folder     string `json:"folder,omitempty"` // Optional
}

// IPRange represents an IP address range
type IPRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ServerConfig holds the BMC server configuration
type ServerConfig struct {
	IPRange IPRange `json:"ip_range"`
}

// Config holds the complete configuration for the virtual BMC
type Config struct {
	VCenter VCenterConfig `json:"vcenter"`
	Server  ServerConfig  `json:"server"`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	config := NewConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate vCenter configuration
	if c.VCenter.IP == "" {
		return fmt.Errorf("vcenter.ip is required")
	}
	if c.VCenter.User == "" {
		return fmt.Errorf("vcenter.user is required")
	}
	if c.VCenter.Password == "" {
		return fmt.Errorf("vcenter.password is required")
	}
	if c.VCenter.Datacenter == "" {
		return fmt.Errorf("vcenter.datacenter is required")
	}

	// Validate server configuration
	if c.Server.IPRange.Start == "" {
		return fmt.Errorf("server.ip_range.start is required")
	}
	if c.Server.IPRange.End == "" {
		return fmt.Errorf("server.ip_range.end is required")
	}

	// Validate IP addresses
	start := net.ParseIP(c.Server.IPRange.Start)
	if start == nil {
		return fmt.Errorf("invalid start IP address: %s", c.Server.IPRange.Start)
	}

	end := net.ParseIP(c.Server.IPRange.End)
	if end == nil {
		return fmt.Errorf("invalid end IP address: %s", c.Server.IPRange.End)
	}

	// Ensure end IP is greater than start IP
	if bytes.Compare(end.To4(), start.To4()) < 0 {
		return fmt.Errorf("end IP must be greater than start IP")
	}

	return nil
}
