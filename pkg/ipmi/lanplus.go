package ipmi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// LANPlus implements the IPMI 2.0 RMCP+ protocol
type LANPlus struct {
	conn          net.Conn
	sessionID    uint32
	sequenceNum  uint32
	managedID    uint32
	authType     uint8
	username     string
	password     string
	timeout      time.Duration
	active       bool
	priv         uint8
	authenticated bool
}

// NewLANPlus creates a new IPMI 2.0 LAN+ session
func NewLANPlus(opts ...Option) *LANPlus {
	l := &LANPlus{
		authType: RMCPPLUS_AUTH_HMAC_SHA1,
		timeout:  time.Second * 5,
		priv:     0x04, // Administrator
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Option defines a function type for setting LANPlus options
type Option func(*LANPlus)

// WithTimeout sets the connection timeout
func WithTimeout(timeout time.Duration) Option {
	return func(l *LANPlus) {
		l.timeout = timeout
	}
}

// WithCredentials sets the username and password
func WithCredentials(username, password string) Option {
	return func(l *LANPlus) {
		l.username = username
		l.password = password
	}
}

// WithPrivilegeLevel sets the privilege level
func WithPrivilegeLevel(priv uint8) Option {
	return func(l *LANPlus) {
		l.priv = priv
	}
}

// Connect establishes a connection to the BMC
func (l *LANPlus) Connect(addr string) error {
	conn, err := net.DialTimeout("udp4", addr, l.timeout)
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	l.conn = conn

	if err := l.openSession(); err != nil {
		l.conn.Close()
		return fmt.Errorf("session open failed: %v", err)
	}

	l.active = true
	return nil
}

// Close closes the IPMI session and connection
func (l *LANPlus) Close() error {
	if !l.active {
		return nil
	}

	if err := l.closeSession(); err != nil {
		return err
	}

	l.active = false
	return l.conn.Close()
}

func (l *LANPlus) openSession() error {
	// Generate random number for session ID
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	l.sessionID = binary.LittleEndian.Uint32(b)

	// Create initial RAKP message
	rakpMsg := []byte{
		0x10, // Message tag
		0x00, // Reserved
		0x00, 0x00, // Maximum privilege level and reserved
		0x00, 0x00, // Reserved session ID
	}

	msg := &RMCPPlusMessage{
		Header: &RMCPPlusHeader{
			AuthType:    l.authType,
			PayloadType: RMCPPLUS_PAYLOAD_RMCPPLUS,
			SessionID:   0, // Must be 0 for session setup
		},
		Payload: rakpMsg,
	}

	// Send initial RAKP message
	if err := l.SendMessage(msg.Payload); err != nil {
		return fmt.Errorf("failed to send RAKP message: %v", err)
	}

	// TODO: Implement full RAKP (Remote Access Key Protocol) handshake
	// This includes:
	// 1. RMCP+ Open Session Request
	// 2. RMCP+ Open Session Response
	// 3. RAKP Message 1
	// 4. RAKP Message 2
	// 5. RAKP Message 3
	// 6. RAKP Message 4

	return errors.New("session establishment not yet implemented")
}

func (l *LANPlus) closeSession() error {
	if !l.authenticated {
		return nil
	}

	// TODO: Implement proper session closure with Close Session command
	return nil
}

func (l *LANPlus) generateAuthCode(data []byte) []byte {
	h := hmac.New(sha1.New, []byte(l.password))
	h.Write(data)
	return h.Sum(nil)
}

// SendMessage sends an IPMI message using RMCP+
func (l *LANPlus) SendMessage(msg []byte) error {
	if !l.active {
		return errors.New("session not active")
	}

	// Create RMCP+ message
	rmcpMsg := &RMCPPlusMessage{
		Header: &RMCPPlusHeader{
			AuthType:       l.authType,
			PayloadType:    RMCPPLUS_PAYLOAD_IPMI,
			SessionID:      l.sessionID,
			SequenceNumber: l.sequenceNum,
		},
		Payload: msg,
	}

	// Format message for sending
	buf := make([]byte, 0, 1024)

	// Add RMCP header
	buf = append(buf,
		0x06, // RMCP Version 1.0
		0x00, // Reserved
		0x00, // Reserved
		0x07, // RMCP+ Message Class
	)

	// Add RMCP+ header
	buf = append(buf, rmcpMsg.Header.AuthType)
	buf = append(buf, rmcpMsg.Header.PayloadType)
	
	// Add Session ID and Sequence Number
	sessionIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sessionIDBytes, rmcpMsg.Header.SessionID)
	buf = append(buf, sessionIDBytes...)

	seqBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(seqBytes, rmcpMsg.Header.SequenceNumber)
	buf = append(buf, seqBytes...)

	// Add payload length
	payloadLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(payloadLen, uint16(len(rmcpMsg.Payload)))
	buf = append(buf, payloadLen...)

	// Add payload
	buf = append(buf, rmcpMsg.Payload...)

	// Add authentication data if required
	if l.authType != RMCPPLUS_AUTH_NONE && l.authenticated {
		authCode := l.generateAuthCode(buf)
		buf = append(buf, authCode...)
	}

	// Send the message
	if _, err := l.conn.Write(buf); err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	l.sequenceNum++
	return nil
}

// ReceiveMessage receives an IPMI message using RMCP+
func (l *LANPlus) ReceiveMessage() ([]byte, error) {
	if !l.active {
		return nil, errors.New("session not active")
	}

	// Set read deadline
	if err := l.conn.SetReadDeadline(time.Now().Add(l.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := l.conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Need at least RMCP header (4 bytes) + RMCP+ header (12 bytes)
	if n < 16 {
		return nil, errors.New("response too short")
	}

	// Verify RMCP header
	if buf[0] != 0x06 || buf[3] != 0x07 {
		return nil, errors.New("invalid RMCP header")
	}

	// Parse RMCP+ header
	authType := buf[4]
	// Verify payload type is IPMI
	if payloadType := buf[5]; payloadType != RMCPPLUS_PAYLOAD_IPMI {
		return nil, fmt.Errorf("unexpected payload type: %d", payloadType)
	}
	sessionID := binary.LittleEndian.Uint32(buf[6:10])
	// Store sequence number for future validation if needed
	l.sequenceNum = binary.LittleEndian.Uint32(buf[10:14])
	payloadLen := binary.LittleEndian.Uint16(buf[14:16])

	// Verify session ID
	if sessionID != l.sessionID {
		return nil, errors.New("invalid session ID")
	}

	// Extract payload
	payloadStart := 16
	payloadEnd := payloadStart + int(payloadLen)
	if payloadEnd > n {
		return nil, errors.New("payload length exceeds message size")
	}

	payload := buf[payloadStart:payloadEnd]

	// Verify authentication if required
	if authType != RMCPPLUS_AUTH_NONE && l.authenticated {
		if payloadEnd+20 > n { // SHA1 produces 20 bytes
			return nil, errors.New("message too short for authentication code")
		}

		receivedAuth := buf[payloadEnd : payloadEnd+20]
		expectedAuth := l.generateAuthCode(buf[:payloadEnd])

		if !hmac.Equal(receivedAuth, expectedAuth) {
			return nil, errors.New("authentication failed")
		}
	}

	return payload, nil
}
