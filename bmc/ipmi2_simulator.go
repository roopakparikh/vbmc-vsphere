package bmc

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
	goipmi "github.com/ooneko/goipmi"
)

// IPMI2Simulator extends the goipmi.Simulator to add IPMI 2.0 support
type IPMI2Simulator struct {
	*goipmi.Simulator
	mutex sync.RWMutex
	// Additional IPMI 2.0 specific fields
	sessionSupport bool
	users          map[string]string // username -> password
	sessions       map[uint32]*ipmi2Session
	log            *logrus.Entry
}

type ipmi2Session struct {
	ID        uint32
	Username  string
	Privilege uint8
}

// NewIPMI2Simulator creates a new IPMI 2.0 simulator instance
func NewIPMI2Simulator(addr net.IP) *IPMI2Simulator {
	logEntry := logrus.WithField("addr", addr)
	logEntry.Info("Creating new IPMI2 simulator")
	udpAddr := &net.UDPAddr{IP: addr, Port: 623} // IPMI default port
	sim := &IPMI2Simulator{
		Simulator:      goipmi.NewSimulator(*udpAddr),
		sessionSupport: true,
		users:          make(map[string]string),
		sessions:       make(map[uint32]*ipmi2Session),
		log:            logEntry,
	}

	// Add default admin user
	sim.users["admin"] = "password"

	// Register handlers for IPMI 2.0 commands
	sim.Simulator.SetHandler(goipmi.NetworkFunctionApp, goipmi.CommandGetAuthCapabilities, sim.handleGetAuthCapabilities)
	sim.Simulator.SetHandler(goipmi.NetworkFunctionApp, goipmi.CommandGetSessionChallenge, sim.handleGetSessionChallenge)
	sim.Simulator.SetHandler(goipmi.NetworkFunctionApp, goipmi.CommandActivateSession, sim.handleActivateSession)
	sim.Simulator.SetHandler(goipmi.NetworkFunctionApp, goipmi.CommandCloseSession, sim.handleCloseSession)

	return sim
}

func (s *IPMI2Simulator) handleGetAuthCapabilities(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Handling GetAuthCapabilities request")
	// IPMI 2.0 authentication capabilities
	return goipmi.CommandCompleted
}

func (s *IPMI2Simulator) handleGetSessionChallenge(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Handling GetSessionChallenge request")
	// For simulator, we accept any challenge request
	return goipmi.CommandCompleted
}

func (s *IPMI2Simulator) handleActivateSession(m *goipmi.Message) goipmi.Response {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sessionID := uint32(len(s.sessions) + 1)
	s.log.WithField("sessionID", sessionID).Debug("Activating new session")
	s.sessions[sessionID] = &ipmi2Session{
		ID:        sessionID,
		Username:  "admin", // Default user for simulator
		Privilege: 0x04,    // Administrator
	}

	return goipmi.CommandCompleted
}

func (s *IPMI2Simulator) handleCloseSession(m *goipmi.Message) goipmi.Response {
	s.log.Debug("Handling CloseSession request")
	return goipmi.CommandCompleted
}

// AddUser adds a new user to the simulator
func (s *IPMI2Simulator) AddUser(username, password string) error {
	s.log.WithField("username", username).Info("Adding new user")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[username]; exists {
		return fmt.Errorf("user %s already exists", username)
	}

	s.users[username] = password
	return nil
}

// Start starts the IPMI simulator
func (s *IPMI2Simulator) Start() error {
	s.log.Info("Starting IPMI2 simulator")
	return s.Simulator.Run()
}

// Stop stops the IPMI simulator
func (s *IPMI2Simulator) Stop() error {
	s.log.Info("Stopping IPMI2 simulator")
	s.Simulator.Stop()
	return nil
}
