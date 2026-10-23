package model

import (
	"time"
)

type Passkey struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	CredentialID    []byte     `json:"credential_id"`
	PublicKey       []byte     `json:"public_key"`
	AttestationType string     `json:"attestation_type"`
	AAGUID          []byte     `json:"aaguid"`
	SignCount       uint32     `json:"sign_count"`
	Transports      []string   `json:"transport"`
	BackupState     bool       `json:"backup_state"`
	DeviceName      string     `json:"device_name"`
	CreatedAt       time.Time  `json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}
