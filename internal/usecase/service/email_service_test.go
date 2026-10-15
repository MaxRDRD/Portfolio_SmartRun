package service_test

import (
	"SmartRun/internal/config"
	"SmartRun/internal/usecase/service"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailService_DevFallback(t *testing.T) {
	ctx := context.Background()

	emailSvc, err := service.NewEmailService(config.EmailConfig{
		Host: "",
	})
	require.NoError(t, err)
	require.NotNil(t, emailSvc)

	err = emailSvc.SendPasswordResetEmail(ctx, "runner@example.com", "http://localhost:3000/reset?token=xyz", "Runner Ivan")
	assert.NoError(t, err)
}
