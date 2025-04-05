package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
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
// LogConfig holds logging configuration
type LogConfig struct {
	Level string `json:"level"` // debug, info, warn, error
}

// NetworkConfig holds network-specific configuration
type NetworkConfig struct {
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
}

// ServerConfig holds the BMC server configuration
type ServerConfig struct {
	IPRange  IPRange      `json:"ip_range"`
	NIC      string       `json:"nic"` // Network interface to bind IPs to
	Network  NetworkConfig `json:"network"`
}

// Config holds the complete configuration for the virtual BMC
type Config struct {
	VCenter VCenterConfig `json:"vcenter"`
	Server  ServerConfig  `json:"server"`
	Logging LogConfig     `json:"logging,omitempty"`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Logging: LogConfig{
			Level: "info", // default log level
		},
		Server: ServerConfig{
			NIC: "eth0", // default network interface
		},
	}
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
// GetLogLevel returns the log level as a logrus.Level
func (c *Config) GetLogLevel() logrus.Level {
	switch c.Logging.Level {
	case "debug":
		return logrus.DebugLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}

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

	// Validate NIC
	if c.Server.NIC == "" {
		return fmt.Errorf("server.nic is required")
	}

	// Validate network configuration
	if c.Server.Network.Netmask == "" {
		return fmt.Errorf("server.network.netmask is required")
	}
	// Validate netmask format
	netmask := net.ParseIP(c.Server.Network.Netmask)
	if netmask == nil {
		return fmt.Errorf("invalid netmask: %s", c.Server.Network.Netmask)
	}

	// Validate gateway if provided
	if c.Server.Network.Gateway != "" {
		gateway := net.ParseIP(c.Server.Network.Gateway)
		if gateway == nil {
			return fmt.Errorf("invalid gateway: %s", c.Server.Network.Gateway)
		}
	}

	// Check if the network interface exists
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to list network interfaces: %v", err)
	}
	nicExists := false
	for _, iface := range interfaces {
		if iface.Name == c.Server.NIC {
			nicExists = true
			break
		}
	}
	if !nicExists {
		return fmt.Errorf("network interface %s does not exist", c.Server.NIC)
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
