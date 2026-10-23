package adapter

import (
	"SmartRun/internal/model"
	"encoding/binary"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUserAdapter struct {
	User     *model.User
	Passkeys []model.Passkey
}

// NewWebAuthnUser оборачивает доменную модель в адаптер
func NewWebAuthnUser(user *model.User, passkeys []model.Passkey) *WebAuthnUserAdapter {
	return &WebAuthnUserAdapter{
		User:     user,
		Passkeys: passkeys,
	}
}

// 1. WebAuthnID: уникальный неизменяемый байтовый идентификатор (User Handle)
func (u *WebAuthnUserAdapter) WebAuthnID() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(u.User.ID))
	return buf
}

// 2. WebAuthnName: логин (Email)
func (u *WebAuthnUserAdapter) WebAuthnName() string {
	return u.User.Email
}

// 3. WebAuthnDisplayName: отображаемое имя (Имя атлета)
func (u *WebAuthnUserAdapter) WebAuthnDisplayName() string {
	if u.User.Name != "" {
		return u.User.Name
	}
	return u.User.Email
}

// 4. WebAuthnCredentials: список зарегистрированных открытых ключей пользователя
func (u *WebAuthnUserAdapter) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(u.Passkeys))
	for _, pk := range u.Passkeys {
		// Конвертируем строки транспортов в protocol.AuthenticatorTransport
		transports := make([]protocol.AuthenticatorTransport, 0, len(pk.Transports))
		for _, t := range pk.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
		creds = append(creds, webauthn.Credential{
			ID:              pk.CredentialID,
			PublicKey:       pk.PublicKey,
			AttestationType: pk.AttestationType,
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				BackupState: pk.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: pk.SignCount,
			},
		})
	}
	return creds
}
