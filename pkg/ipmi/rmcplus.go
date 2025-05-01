package ipmi

// RMCP+ related constants for IPMI v2.0
const (
	// RMCP+ Message Status and Authentication Types
	RMCPPLUS_STATUS_NO_ERRORS     = 0x00
	RMCPPLUS_STATUS_INSUFFICIENT  = 0x01
	RMCPPLUS_STATUS_UNAUTHORIZED  = 0x02
	RMCPPLUS_STATUS_UNAVAILABLE  = 0x03
	RMCPPLUS_STATUS_NOT_SUPPORTED = 0x04

	// Authentication Types
	RMCPPLUS_AUTH_NONE      = 0x00
	RMCPPLUS_AUTH_HMAC_SHA1 = 0x01
	RMCPPLUS_AUTH_HMAC_MD5  = 0x02

	// Payload Types
	RMCPPLUS_PAYLOAD_IPMI       = 0x00
	RMCPPLUS_PAYLOAD_SOL        = 0x01
	RMCPPLUS_PAYLOAD_OEM        = 0x02
	RMCPPLUS_PAYLOAD_RMCPPLUS   = 0x03
)

// RMCPPlusHeader represents the RMCP+ session header
type RMCPPlusHeader struct {
	AuthType       uint8
	PayloadType    uint8
	SessionID      uint32
	SequenceNumber uint32
}

// NewRMCPPlusHeader creates a new RMCP+ header with default values
func NewRMCPPlusHeader() *RMCPPlusHeader {
	return &RMCPPlusHeader{
		AuthType:       RMCPPLUS_AUTH_NONE,
		PayloadType:    RMCPPLUS_PAYLOAD_IPMI,
		SessionID:      0,
		SequenceNumber: 0,
	}
}

// RMCPPlusMessage represents a complete RMCP+ message
type RMCPPlusMessage struct {
	Header  *RMCPPlusHeader
	Payload []byte
}

// NewRMCPPlusMessage creates a new RMCP+ message with the given payload
func NewRMCPPlusMessage(payload []byte) *RMCPPlusMessage {
	return &RMCPPlusMessage{
		Header:  NewRMCPPlusHeader(),
		Payload: payload,
	}
}
