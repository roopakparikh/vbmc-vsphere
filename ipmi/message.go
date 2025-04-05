package ipmi

import "fmt"

// IPMI Authentication Types
const (
	AuthTypeNone = iota
	AuthTypeMD2
	AuthTypeMD5
	AuthTypePassword
	AuthTypeOEM
)

// IPMI Privilege Levels
const (
	PrivLevelNone = iota
	PrivLevelCallback
	PrivLevelUser
	PrivLevelOperator
	PrivLevelAdmin
	PrivLevelOEM
)

// IPMI Commands
const (
	CommandGetDeviceID              = 0x01
	CommandGetAuthCapabilities      = 0x38
	CommandGetSessionChallenge      = 0x39
	CommandActivateSession          = 0x3a
	CommandSetSessionPrivilegeLevel = 0x3b
	CommandCloseSession             = 0x3c
	CommandChassisControl          = 0x02
	CommandChassisStatus           = 0x01
	CommandSetSystemBootOptions     = 0x08
	CommandGetSystemBootOptions     = 0x09
)

// IPMI Network Functions
const (
	NetworkFunctionChassis = 0x00
	NetworkFunctionApp     = 0x06
)

// IPMI Completion Codes
const (
	CompletionCodeNormal           = 0x00
	CompletionCodeNodeBusy         = 0xc0
	CompletionCodeInvalidCommand   = 0xc1
	CompletionCodeInvalidLUN       = 0xc2
	CompletionCodeTimeout          = 0xc3
	CompletionCodeOutOfSpace       = 0xc4
	CompletionCodeInvalidReserv    = 0xc5
	CompletionCodeDataTruncated    = 0xc6
	CompletionCodeInvalidLength    = 0xc7
	CompletionCodeLengthExceeded   = 0xc8
	CompletionCodeInvalidField     = 0xc9
	CompletionCodeInvalidChecksum  = 0xca
	CompletionCodeInvalidSession   = 0xcb
	CompletionCodeWriteProtect     = 0xcc
	CompletionCodeInvalidType      = 0xcd
	CompletionCodeInvalidAuthType  = 0xce
	CompletionCodeInvalidHandle    = 0xcf
	CompletionCodeInvalidReserved  = 0xd0
	CompletionCodeUnknownError     = 0xff
)

// RMCPHeader represents the RMCP message header
type RMCPHeader struct {
	Version  uint8
	Reserved uint8
	Seq      uint8
	Class    uint8
}

// Message represents a complete IPMI message
type Message struct {
	RMCPHeader
	AuthType  uint8
	SessionID uint32
	Sequence  uint32
	Command   uint8
	Data      []byte
}

// Pack converts the Message into a byte slice for transmission
func (m *Message) Pack() ([]byte, error) {
	// Pack RMCP header
	header := []byte{
		m.Version,
		m.Reserved,
		m.Seq,
		m.Class,
	}

	// Pack IPMI session header
	session := []byte{
		m.AuthType,
		0x00, // Reserved
		0x00, // Reserved
		0x00, // Reserved
		0x00, // Reserved
		0x00, // Reserved
		byte(m.SessionID >> 24),
		byte(m.SessionID >> 16),
		byte(m.SessionID >> 8),
		byte(m.SessionID),
	}

	// Pack command and data
	cmd := []byte{m.Command}

	// Combine all parts
	return append(append(header, session...), append(cmd, m.Data...)...), nil
}

// Unpack parses a byte slice into a Message
func (m *Message) Unpack(data []byte) error {
	if len(data) < 15 { // Minimum size for RMCP + IPMI session header
		return fmt.Errorf("message too short")
	}

	// Unpack RMCP header
	m.Version = data[0]
	m.Reserved = data[1]
	m.Seq = data[2]
	m.Class = data[3]

	// Verify RMCP version and class
	if m.Version != 0x06 { // RMCP v1.0
		return fmt.Errorf("unsupported RMCP version: %d", m.Version)
	}
	if m.Class != 0x07 { // IPMI class
		return fmt.Errorf("unsupported message class: %d", m.Class)
	}

	// Unpack IPMI session header
	m.AuthType = data[4]
	m.SessionID = uint32(data[10])<<24 | uint32(data[11])<<16 | uint32(data[12])<<8 | uint32(data[13])

	// Unpack command and data
	if len(data) < 15 {
		return fmt.Errorf("message too short for command")
	}
	m.Command = data[14]
	if len(data) > 15 {
		m.Data = data[15:]
	}

	return nil
}
