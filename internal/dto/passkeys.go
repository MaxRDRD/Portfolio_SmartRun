package dto

import (
	"time"

	"github.com/go-webauthn/webauthn/protocol"
)

type PasskeyLoginBeginRequest struct {
	Email string `json:"email,omitempty"`
}

type PasskeyLoginBeginResponse struct {
	SessionID string                        `json:"session_id"`
	Options   *protocol.CredentialAssertion `json:"options"`
}

type PasskeyResponse struct {
	ID         int64      `json:"id"`
	DeviceName string     `json:"device_name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
