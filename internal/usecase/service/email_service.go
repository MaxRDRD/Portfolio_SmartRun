package service

import (
	"SmartRun/internal/config"
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"time"

	"github.com/wneessen/go-mail"
)

var templates embed.FS

type EmailService interface {
	SendPasswordResetEmail(ctx context.Context, to string, resetLink string, userName string) error
	// Можно добавить другие методы позже: SendWelcome, Send2FA, etc.
}

type emailService struct {
	client   *mail.Client
	from     string
	fromName string
}

func NewEmailService(cfg config.EmailConfig) (EmailService, error) {
	clientOpts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password),
		mail.WithSMTPAuth(mail.SMTPAuthPlain), // или mail.SMTPAuthLogin, если нужно
	}

	if cfg.UseTLS {
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}

	client, err := mail.NewClient(cfg.Host, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail client: %w", err)
	}

	// Опциональная проверка: пробуем установить соединение один раз
	// (если не нужно — просто удали этот блок)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.DialWithContext(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("smtp connection test failed at startup: %w", err)
	}
	_ = client.Close() // сразу закрываем тестовое соединение

	return &emailService{
		client:   client,
		from:     cfg.From,
		fromName: cfg.FromName,
	}, nil
}

func (s *emailService) SendPasswordResetEmail(ctx context.Context, to, resetLink, userName string) error {
	msg := mail.NewMsg(
		mail.WithCharset(mail.CharsetUTF8),
	)

	// Установка From — два варианта (выбери один)
	// Вариант 1: простой (только адрес)
	// if err := msg.From(s.from); err != nil {
	// 	return fmt.Errorf("invalid from address: %w", err)
	// }

	// Вариант 2: с именем (рекомендуется — отображается как "SmartRun <noreply@...>")
	if err := msg.FromFormat(s.fromName, s.from); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}

	// Получатель
	if err := msg.To(to); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}

	// Тема
	msg.Subject("Сброс пароля в SmartRun")

	// Plain text версия (fallback)
	plainText := fmt.Sprintf(`Привет, %s!

Перейдите по ссылке, чтобы сбросить пароль:
%s

Ссылка действительна 60 минут.
Если это не вы — просто проигнорируйте письмо.

С уважением,
SmartRun Team`, userName, resetLink)

	// Устанавливаем основной body как plain text
	msg.SetBodyString(mail.TypeTextPlain, plainText)

	// HTML версия (если шаблон существует)
	tmpl, err := template.ParseFS(templates, "templates/reset_password.html")
	if err == nil {
		var htmlBody bytes.Buffer
		data := struct {
			Name     string
			Link     string
			Duration string
		}{
			Name:     userName,
			Link:     resetLink,
			Duration: "60 минут",
		}

		if execErr := tmpl.Execute(&htmlBody, data); execErr == nil {
			// Добавляем альтернативную часть (multipart/alternative)
			msg.AddAlternativeString(mail.TypeTextHTML, htmlBody.String())
		}
		// если ошибка рендера — остаётся только plain text
	}

	// Отправка с таймаутом
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := s.client.DialAndSendWithContext(sendCtx, msg); err != nil {
		return fmt.Errorf("failed to send reset email to %s: %w", to, err)
	}

	return nil
}
