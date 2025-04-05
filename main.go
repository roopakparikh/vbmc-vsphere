package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/vbmc-vsphere/config"
	"github.com/vbmc-vsphere/ipmi"
	"github.com/vbmc-vsphere/vsphere"
)

// ipRange calculates the number of IP addresses between start and end
func ipRange(start, end net.IP) int64 {
	var i int64
	for i = 1; ; i++ {
		if start.Equal(end) {
			break
		}
		incrementIP(start)
	}
	return i
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}
}

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	log := logrus.New()
	log.SetLevel(cfg.GetLogLevel())
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.Info("Starting vBMC-vSphere service")
	log.Infof("Using config file: %s", *configFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create vSphere client
	log.Info("Connecting to vSphere...")
	vsClient, err := vsphere.NewClient(ctx, cfg.VCenter.IP, cfg.VCenter.User, cfg.VCenter.Password, cfg.VCenter.Datacenter)
	if err != nil {
		log.Fatalf("Failed to create vSphere client: %v", err)
	}

	// Get list of VMs
	log.Infof("Retrieving VMs from folder: %s", cfg.VCenter.Folder)
	vms, err := vsClient.GetVMs(ctx, cfg.VCenter.Folder)
	log.Infof("Found %d VMs", len(vms))
	if err != nil {
		log.Fatalf("Failed to get VMs: %v", err)
	}

	// Create IP address pool
	startIP := net.ParseIP(cfg.Server.IPRange.Start).To4()
	endIP := net.ParseIP(cfg.Server.IPRange.End).To4()

	// Calculate number of available IPs
	ipCount := ipRange(startIP, endIP)
	if ipCount < int64(len(vms)) {
		log.Fatalf("Not enough IP addresses in range for all VMs. Need %d, have %d", len(vms), ipCount)
	}

	// Create IPMI servers for each VM
	var wg sync.WaitGroup
	servers := make([]*ipmi.Server, len(vms))

	// Parse netmask
	netmask := net.ParseIP(cfg.Server.Network.Netmask)
	if netmask == nil {
		log.Fatalf("Failed to parse netmask: %s", cfg.Server.Network.Netmask)
	}

	// Track used IPs to avoid conflicts
	usedIPs := make(map[string]bool)

	currentIP := make(net.IP, len(startIP))
	copy(currentIP, startIP)

	for i, vm := range vms {
		// Check if IP is already in use
		ipStr := currentIP.String()
		for usedIPs[ipStr] {
			incrementIP(currentIP)
			ipStr = currentIP.String()
			if currentIP.Equal(endIP) {
				log.Fatalf("No more available IPs in range")
			}
		}
		usedIPs[ipStr] = true

		server := ipmi.NewServer(vm, vsClient, currentIP, netmask, cfg.Server.NIC)
		servers[i] = server

		wg.Add(1)
		go func(s *ipmi.Server) {
			defer wg.Done()
			if err := s.Start(ctx); err != nil {
				log.Errorf("Failed to start IPMI server: %v", err)
			}
		}(server)

		vmName := vm.Name()
		log.Infof("Started virtual BMC for VM %s on IP %s", vmName, currentIP)

		// Increment IP for next VM
		incrementIP(currentIP)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("Shutting down...")
	cancel()

	// Stop all servers
	for _, server := range servers {
		server.Stop()
	}

	wg.Wait()
	log.Info("Shutdown complete")
}
