package bmc

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/object"
	"github.com/vbmc-vsphere/vsphere"
	goipmi "github.com/ooneko/goipmi"
)

// Server represents an IPMI server instance
type Server struct {
	vm       *object.VirtualMachine
	vsClient *vsphere.Client
	ipmiServer *IPMI2Simulator
	ip       net.IP
	netmask  net.IP
	nic      string
	log      *logrus.Entry
}

// NewServer creates a new IPMI server instance
func NewServer(vm *object.VirtualMachine, vsClient *vsphere.Client, ip net.IP, netmask net.IP, nic string) *Server {
	s := &Server{
		vm:       vm,
		vsClient: vsClient,
		ip:       ip,
		netmask:  netmask,
		nic:      nic,
		log:      logrus.WithField("vm", vm.Name()),
	}

	return s
}

// handleChassisControl handles IPMI chassis control commands
func (s *Server) handleChassisControl(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Handling chassis control command")

	// Parse command
	req := &goipmi.ChassisControlRequest{}
	if err := m.Request(req); err != nil {
		s.log.Errorf("Failed to parse chassis control request: %v", err)
		return goipmi.ErrInvalidCommand
	}

	ctx := context.Background()
	switch req.ChassisControl {
	case goipmi.ControlPowerDown: // PowerDown
		s.log.Info("Power down command received")
		if err := s.vsClient.PowerOffVM(ctx, s.vm); err != nil {
			s.log.Errorf("Failed to power off VM: %v", err)
			return goipmi.ErrUnspecified
		}
	case goipmi.ControlPowerUp: // PowerUp
		s.log.Info("Power up command received")
		if err := s.vsClient.PowerOnVM(ctx, s.vm); err != nil {
			s.log.Errorf("Failed to power on VM: %v", err)
			return goipmi.ErrUnspecified
		}
	case goipmi.ControlPowerHardReset: // HardReset
		s.log.Info("Reset command received")
		if err := s.vsClient.ResetVM(ctx, s.vm); err != nil {
			s.log.Errorf("Failed to reset VM: %v", err)
			return goipmi.ErrUnspecified
		}
	case goipmi.ControlPowerCycle: // PowerCycle
		s.log.Info("Power cycle command received")
		// Power cycle is implemented as power off followed by power on
		if err := s.vsClient.PowerOffVM(ctx, s.vm); err != nil {
			s.log.Errorf("Failed to power off VM during cycle: %v", err)
			return goipmi.ErrUnspecified
		}
		if err := s.vsClient.PowerOnVM(ctx, s.vm); err != nil {
			s.log.Errorf("Failed to power on VM during cycle: %v", err)
			return goipmi.ErrUnspecified
		}
	default:
		s.log.Warnf("Unsupported chassis control command: %v", req.ChassisControl)
		return goipmi.ErrInvalidCommand
	}

	return goipmi.CommandCompleted	
}

// handleGetChassisStatus handles IPMI get chassis status commands
func (s *Server) handleGetChassisStatus(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Getting chassis status")

	ctx := context.Background()
	powerState, err := s.vsClient.GetVMPowerState(ctx, s.vm)
	if err != nil {
		s.log.Errorf("Failed to get power state: %v", err)
		return goipmi.ErrUnspecified
	}

	// Return chassis status
	var powerStateByte byte
	if powerState == "poweredOn" {
		powerStateByte = goipmi.SystemPower
	}

	return &goipmi.ChassisStatusResponse{
		CompletionCode: goipmi.CommandCompleted,
		PowerState:     powerStateByte,
	}
}

// handleSetSystemBootOptions handles IPMI set system boot options commands
func (s *Server) handleSetSystemBootOptions(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Setting system boot options")

	// Parse boot options
	req := &goipmi.SetSystemBootOptionsRequest{}
	if err := m.Request(req); err != nil {
		s.log.Errorf("Failed to parse boot options request: %v", err)
		return goipmi.ErrUnspecified
	}

	// Check if this is a boot flags parameter
	if req.Param != goipmi.BootParamBootFlags {
		return &goipmi.SetSystemBootOptionsResponse{CompletionCode: goipmi.CommandCompleted} // Ignore non-boot flags parameters
	}

	// Map IPMI boot device to vSphere boot device
	var bootDevice vsphere.BootDevice
	switch goipmi.BootDevice(req.Data[1]) { // Mask out persistent/EFI bits
	case goipmi.BootDeviceNone: // No override
		return &goipmi.SetSystemBootOptionsResponse{CompletionCode: goipmi.CommandCompleted}
	case goipmi.BootDeviceDisk:
		bootDevice = vsphere.BootDeviceHDD
	case goipmi.BootDeviceCdrom:
		bootDevice = vsphere.BootDeviceCDROM
	case goipmi.BootDevicePxe:
		bootDevice = vsphere.BootDevicePXE
	case goipmi.BootDeviceFloppy:
		bootDevice = vsphere.BootDeviceFloppy
	default:
		s.log.Warnf("Unsupported boot device: %v", req.Data[1])
		return goipmi.ErrInvalidObjCommand
	}

	// Set the boot device
	ctx := context.Background()
	if err := s.vsClient.SetNextBoot(ctx, s.vm, bootDevice); err != nil {
		s.log.Errorf("Failed to set boot device: %v", err)
		return goipmi.ErrUnspecified
	}

	return &goipmi.SetSystemBootOptionsResponse{CompletionCode: goipmi.CommandCompleted}
}

// Start starts the IPMI server
// configureIP configures the IP address on the specified network interface
func (s *Server) configureIP() error {
	// Check if IP already exists
	checkCmd := exec.Command("ip", "addr", "show", "dev", s.nic)
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check IP configuration on %s: %v - %s", 
			s.nic, err, string(checkOutput))
	}

	// Check if our IP is already in the output
	if strings.Contains(string(checkOutput), s.ip.String()) {
		s.log.Infof("IP %s already configured on interface %s, skipping configuration", 
			s.ip.String(), s.nic)
		return nil
	}

	// Use ip command to add IP address
	cmd := exec.Command("ip", "addr", "add", 
		fmt.Sprintf("%s/%s", s.ip.String(), s.netmask.String()), 
		"dev", s.nic)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to configure IP %s on %s: %v - %s", 
			s.ip.String(), s.nic, err, string(output))
	}

	s.log.Infof("Configured IP %s with netmask %s on interface %s", 
		s.ip.String(), s.netmask.String(), s.nic)
	return nil
}

// cleanupIP removes the IP address from the network interface
func (s *Server) cleanupIP() error {
	if s.ip == nil || s.nic == "" {
		return nil
	}

	cmd := exec.Command("ip", "addr", "del", 
		fmt.Sprintf("%s/%s", s.ip.String(), s.netmask.String()), 
		"dev", s.nic)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.log.Errorf("Failed to remove IP %s from %s: %v - %s", 
			s.ip.String(), s.nic, err, string(output))
		return err
	}

	s.log.Infof("Removed IP %s from interface %s", s.ip.String(), s.nic)
	return nil
}

func (s *Server) Start(ctx context.Context) error {
	// Configure IP address on the interface
	if err := s.configureIP(); err != nil {
		return fmt.Errorf("failed to configure IP: %v", err)
	}

	// Create IPMI server
	s.ipmiServer = NewIPMI2Simulator(s.ip)

	// Register handlers
	s.ipmiServer.SetHandler(goipmi.NetworkFunctionChassis, goipmi.CommandChassisControl, s.handleChassisControl)
	s.ipmiServer.SetHandler(goipmi.NetworkFunctionChassis, goipmi.CommandChassisStatus, s.handleGetChassisStatus)
	s.ipmiServer.SetHandler(goipmi.NetworkFunctionChassis, goipmi.CommandSetSystemBootOptions, s.handleSetSystemBootOptions)

	// Start server
	if err := s.ipmiServer.Start(); err != nil {
		return fmt.Errorf("failed to start IPMI server: %v", err)
	}

	return nil
}

// Stop stops the IPMI server
func (s *Server) Stop() error {
	// Stop the IPMI simulator
	if s.ipmiServer != nil {
		s.ipmiServer.Stop()
	}

	// Clean up the IP configuration
	if err := s.cleanupIP(); err != nil {
		return fmt.Errorf("failed to cleanup IP configuration: %v", err)
	}

	s.log.Info("IPMI server stopped")
	return nil
}
