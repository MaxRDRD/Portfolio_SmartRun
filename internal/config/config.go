package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type AuthConfig struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	JWTSecret       string
	PublicURL       string
	Email           EmailConfig
}

type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	UseTLS   bool // true для 465/587 с STARTTLS или implicit TLS
}

type LoggerConfig struct {
	Level    string
	Output   string
	FilePath string
}

type RedisConfig struct {
	URL string
}

type RabbitMQConfig struct {
	URL string
}

type WebAuthnConfig struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
}

type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Logger   LoggerConfig
	Auth     AuthConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	WebAuthn WebAuthnConfig
}

type HTTPConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	TLSEnabled   bool
	TLSCertFile  string
	TLSKeyFile   string
}

// Address возвращает адрес в формате host:port для http.Server
func (h *HTTPConfig) Address() string {
	return net.JoinHostPort(h.Host, h.Port)
}

type PostgresConfig struct {
	Host               string
	Port               string
	User               string
	Password           string
	DBName             string
	SSLMode            string
	ConnStringOverride string
	MaxConns           int32
	MinConns           int32
}

// ConnString формирует строку подключения к PostgreSQL
func (p *PostgresConfig) ConnString() string {
	if p.ConnStringOverride != "" {
		return p.ConnStringOverride
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode,
	)
}

// Load загружает конфигурацию из переменных окружения и .env файлов
func Load() (*Config, error) {
	// Подгружаем .env при наличии
	loadDotEnv()

	cfg := &Config{
		HTTP: HTTPConfig{
			Host:         getEnv("HTTP_HOST", "0.0.0.0"),
			Port:         getEnv("HTTP_PORT", getEnv("PORT", "8080")),
			ReadTimeout:  time.Duration(getEnvAsInt("HTTP_READ_TIMEOUT", 15)) * time.Second,
			WriteTimeout: time.Duration(getEnvAsInt("HTTP_WRITE_TIMEOUT", 15)) * time.Second,
			IdleTimeout:  time.Duration(getEnvAsInt("HTTP_IDLE_TIMEOUT", 60)) * time.Second,
			TLSEnabled:   getEnvAsBool("TLS_ENABLED", false),
			TLSCertFile:  getEnv("TLS_CERT_FILE", "certs/server.crt"),
			TLSKeyFile:   getEnv("TLS_KEY_FILE", "certs/server.key"),
		},
		Postgres: PostgresConfig{
			Host:               getEnv("POSTGRES_HOST", "localhost"),
			Port:               getEnv("POSTGRES_PORT", "5432"),
			User:               getEnv("POSTGRES_USER", "postgres"),
			Password:           getEnv("POSTGRES_PASSWORD", "postgres"),
			DBName:             getEnv("POSTGRES_DB", "smartrun"),
			SSLMode:            getEnv("POSTGRES_SSLMODE", "disable"),
			ConnStringOverride: getEnv("CONN_STRING", getEnv("DATABASE_URL", "")),
			MaxConns:           int32(getEnvAsInt("POSTGRES_MAX_CONNS", 10)),
			MinConns:           int32(getEnvAsInt("POSTGRES_MIN_CONNS", 2)),
		},
		Logger: LoggerConfig{
			Level:    getEnv("LOG_LEVEL", "info"),
			Output:   getEnv("LOG_OUTPUT", "stdout"),
			FilePath: getEnv("LOG_FILE_PATH", "logs/app.log"),
		},
		Auth: AuthConfig{
			AccessTokenTTL:  time.Duration(getEnvAsInt("AUTH_ACCESS_TOKEN_TTL_MINUTES", 15)) * time.Minute,
			RefreshTokenTTL: time.Duration(getEnvAsInt("AUTH_REFRESH_TOKEN_TTL_HOURS", 720)) * time.Hour,
			JWTSecret:       getEnv("JWT_SECRET", "super-secret-key"),
			PublicURL:       getEnv("APP_PUBLIC_URL", "http://localhost:3000"),
			Email: EmailConfig{
				Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
				Port:     getEnvAsInt("SMTP_PORT", 587),
				Username: getEnv("SMTP_USERNAME", ""),
				Password: getEnv("SMTP_PASSWORD", ""),
				From:     getEnv("EMAIL_FROM", "noreply@smartrun.app"),
				FromName: getEnv("EMAIL_FROM_NAME", "SmartRun"),
				UseTLS:   getEnvAsBool("SMTP_USE_TLS", false),
			},
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", ""),
		},
		RabbitMQ: RabbitMQConfig{
			URL: getEnv("RABBITMQ_URL", "amqp://smartrun:smartrun@localhost:5672/"),
		},
		WebAuthn: WebAuthnConfig{
			RPDisplayName: getEnv("WEBAUTHN_RP_DISPLAY_NAME", "SmartRun"),
			RPID:          getEnv("WEBAUTHN_RP_ID", "localhost"),
			RPOrigins: strings.Split(
				getEnv("WEBAUTHN_RP_ORIGINS", getEnv("APP_PUBLIC_URL", "http://localhost:3000")+",http://localhost:8080"),
				",",
			),
		},
	}

	return cfg, nil
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvAsInt возвращает числовое значение переменной окружения или значение по умолчанию
func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// getEnvAsBool возвращает булево значение переменной окружения или значение по умолчанию
func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// loadDotEnv пытается прочитать .env файл из текущей или родительских директорий
func loadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)

			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
		break
	}
}
