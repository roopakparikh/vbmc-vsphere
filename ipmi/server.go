package ipmi

import (
	"context"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/object"
	"github.com/vbmc-vsphere/vsphere"
)

// Server represents an IPMI server instance
type Server struct {
	vm       *object.VirtualMachine
	vsClient *vsphere.Client
	conn     *net.UDPConn
	ip       net.IP
	log      *logrus.Entry
}

// NewServer creates a new IPMI server instance
func NewServer(vm *object.VirtualMachine, vsClient *vsphere.Client, ip net.IP) *Server {
	return &Server{
		vm:       vm,
		vsClient: vsClient,
		ip:       ip,
		log:      logrus.WithField("vm", vm.Name()),
	}
}

// Start starts the IPMI server
func (s *Server) Start(ctx context.Context) error {
	addr := &net.UDPAddr{
		Port: 623, // Standard IPMI port
		IP:   s.ip,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s:623: %v", s.ip, err)
	}
	s.conn = conn
	s.log.Infof("IPMI server listening on %s:623", s.ip)

	go s.handleConnections(ctx)
	return nil
}

func (s *Server) handleConnections(ctx context.Context) {
	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				s.log.Errorf("Failed to read from UDP: %v", err)
				continue
			}

			go s.handleIPMIRequest(ctx, buffer[:n], remoteAddr)
		}
	}
}

func (s *Server) handleIPMIRequest(ctx context.Context, data []byte, remoteAddr *net.UDPAddr) {
	// Basic IPMI message parsing
	if len(data) < 4 {
		s.log.Error("Invalid IPMI message: too short")
		return
	}

	// IPMI message parsing
	if len(data) < 5 { // Need at least 5 bytes for command and parameters
		s.log.Error("Invalid IPMI message: too short")
		return
	}

	// Extract IPMI command and parameters
	command := data[3]
	params := data[4:]

	response := s.processIPMICommand(ctx, command, params)

	// Send response
	_, err := s.conn.WriteToUDP(response, remoteAddr)
	if err != nil {
		s.log.Errorf("Failed to send response: %v", err)
	}
}

func (s *Server) processIPMICommand(ctx context.Context, command byte, params []byte) []byte {
	switch command {
	case 0x01: // Get Device ID
		return []byte{0x00, 0x01, 0x00, 0x00} // Simple response

	case 0x02: // Get Power State
		powerState, err := s.vsClient.GetVMPowerState(ctx, s.vm)
		if err != nil {
			s.log.Errorf("Failed to get power state: %v", err)
			return []byte{0x01} // Error response
		}
		if powerState == "poweredOn" {
			return []byte{0x00, 0x01} // Powered on
		}
		return []byte{0x00, 0x00} // Powered off

	case 0x03: // Power On
		err := s.vsClient.PowerOnVM(ctx, s.vm)
		if err != nil {
			s.log.Errorf("Failed to power on VM: %v", err)
			return []byte{0x01} // Error response
		}
		return []byte{0x00} // Success

	case 0x04: // Power Off
		err := s.vsClient.PowerOffVM(ctx, s.vm)
		if err != nil {
			s.log.Errorf("Failed to power off VM: %v", err)
			return []byte{0x01} // Error response
		}
		return []byte{0x00} // Success

	case 0x08: // Set Boot Device
		if len(params) < 1 {
			s.log.Error("Invalid boot device command: no parameters")
			return []byte{0x01} // Error response
		}

		// Map IPMI boot device to vSphere boot device
		var bootDevice vsphere.BootDevice
		switch params[0] & 0x3F { // Mask out persistent/EFI bits
		case 0x00: // No override
			return []byte{0x00} // Success
		case 0x04: // Boot from HDD
			bootDevice = vsphere.BootDeviceHDD
		case 0x14: // Boot from CD/DVD
			bootDevice = vsphere.BootDeviceCDROM
		case 0x20: // Boot from PXE
			bootDevice = vsphere.BootDevicePXE
		case 0x3C: // Boot from Floppy
			bootDevice = vsphere.BootDeviceFloppy
		default:
			s.log.Warnf("Unsupported boot device: %02x", params[0])
			return []byte{0x01} // Error response
		}

		// Set the boot device
		err := s.vsClient.SetNextBoot(ctx, s.vm, bootDevice)
		if err != nil {
			s.log.Errorf("Failed to set boot device: %v", err)
			return []byte{0x01} // Error response
		}
		return []byte{0x00} // Success

	default:
		s.log.Warnf("Unsupported IPMI command: %02x", command)
		return []byte{0x01} // Error response
	}
}

// Stop stops the IPMI server
func (s *Server) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}
