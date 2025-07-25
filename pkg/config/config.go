// Package config provides centralized configuration management for the Fern Platform
// It follows the twelve-factor app methodology for configuration management
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete platform configuration
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Services   ServicesConfig   `mapstructure:"services"`
	Redis      RedisConfig      `mapstructure:"redis"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Host            string        `mapstructure:"host"`
	ReadTimeout     time.Duration `mapstructure:"readTimeout"`
	WriteTimeout    time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout     time.Duration `mapstructure:"idleTimeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout"`
	TLS             TLSConfig     `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"certFile"`
	KeyFile  string `mapstructure:"keyFile"`
}

type DatabaseConfig struct {
	Uri             string        `mapstructure:"uri"` // Deprecated, use individual fields instead
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	TimeZone        string        `mapstructure:"timezone"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connMaxIdleTime"`
}

type AuthConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	JWTSecret     string        `mapstructure:"jwtSecret"`
	JWKSUrl       string        `mapstructure:"jwksUrl"`
	Issuer        string        `mapstructure:"issuer"`
	Audience      string        `mapstructure:"audience"`
	TokenExpiry   time.Duration `mapstructure:"tokenExpiry"`
	RefreshExpiry time.Duration `mapstructure:"refreshExpiry"`
	OAuth         OAuthConfig   `mapstructure:"oauth"`
}

type OAuthConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	ClientID     string   `mapstructure:"clientId"`
	ClientSecret string   `mapstructure:"clientSecret"`
	RedirectURL  string   `mapstructure:"redirectUrl"`
	Scopes       []string `mapstructure:"scopes"`

	// OAuth 2.0/OpenID Connect endpoints (required for any provider)
	AuthURL     string `mapstructure:"authUrl"`     // Authorization endpoint
	TokenURL    string `mapstructure:"tokenUrl"`    // Token endpoint
	UserInfoURL string `mapstructure:"userInfoUrl"` // UserInfo endpoint
	JWKSUrl     string `mapstructure:"jwksUrl"`     // JWKS endpoint for token validation
	IssuerURL   string `mapstructure:"issuerUrl"`   // OpenID Connect Discovery URL (optional)
	LogoutURL   string `mapstructure:"logoutUrl"`   // Logout endpoint (optional)

	// User and role mapping
	AdminUsers       []string          `mapstructure:"adminUsers"`       // List of admin user emails/IDs
	AdminGroups      []string          `mapstructure:"adminGroups"`      // List of admin groups from token claims
	UserRoleMapping  map[string]string `mapstructure:"userRoleMapping"`  // Map specific users to roles
	GroupRoleMapping map[string]string `mapstructure:"groupRoleMapping"` // Map groups to roles

	// Role group names (configurable)
	AdminGroupName   string `mapstructure:"adminGroupName"`   // Name of admin group (default: "admin")
	ManagerGroupName string `mapstructure:"managerGroupName"` // Name of manager group (default: "manager")
	UserGroupName    string `mapstructure:"userGroupName"`    // Name of user group (default: "user")

	// Token claim field mappings (customize based on your provider)
	UserIDField string `mapstructure:"userIdField"` // Field in token containing user ID (default: "sub")
	EmailField  string `mapstructure:"emailField"`  // Field containing email (default: "email")
	NameField   string `mapstructure:"nameField"`   // Field containing display name (default: "name")
	GroupsField string `mapstructure:"groupsField"` // Field containing user groups (default: "groups")
	RolesField  string `mapstructure:"rolesField"`  // Field containing user roles (default: "roles")
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Structured bool   `mapstructure:"structured"`
}

type ServicesConfig struct {
	Reporter ServiceEndpoint `mapstructure:"reporter"`
	Mycelium ServiceEndpoint `mapstructure:"mycelium"`
	UI       ServiceEndpoint `mapstructure:"ui"`
}

type ServiceEndpoint struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	URL  string `mapstructure:"url"`
}

type RedisConfig struct {
	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	Password    string        `mapstructure:"password"`
	DB          int           `mapstructure:"db"`
	PoolSize    int           `mapstructure:"poolSize"`
	IdleTimeout time.Duration `mapstructure:"idleTimeout"`
}

type LLMConfig struct {
	DefaultProvider string                 `mapstructure:"defaultProvider"`
	Providers       map[string]LLMProvider `mapstructure:"providers"`
	CacheEnabled    bool                   `mapstructure:"cacheEnabled"`
	CacheTTL        time.Duration          `mapstructure:"cacheTTL"`
	MaxTokens       int                    `mapstructure:"maxTokens"`
	Temperature     float32                `mapstructure:"temperature"`
}

type LLMProvider struct {
	Type    string `mapstructure:"type"`
	APIKey  string `mapstructure:"apiKey"`
	BaseURL string `mapstructure:"baseUrl"`
	Model   string `mapstructure:"model"`
	Enabled bool   `mapstructure:"enabled"`
}

type MonitoringConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
	Health  HealthConfig  `mapstructure:"health"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

type TracingConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"serviceName"`
	Endpoint    string `mapstructure:"endpoint"`
}

type HealthConfig struct {
	Path     string        `mapstructure:"path"`
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

var globalConfig *Config

// Manager handles configuration initialization and management
type Manager struct {
	config *Config
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// Load loads configuration from multiple sources with precedence:
// 1. Environment variables (highest)
// 2. Config file
// 3. Default values (lowest)
func (m *Manager) Load(configPath string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath("./config")
		viper.AddConfigPath("../config")
		viper.AddConfigPath("../../config")
		viper.AddConfigPath("/etc/fern-platform")
	}

	// Set defaults
	m.setDefaults()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; continue with defaults and env vars
	}

	// Environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if err := m.bindEnvVars(); err != nil {
		return fmt.Errorf("failed to bind environment variables: %w", err)
	}

	// Unmarshal into config struct
	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := m.validate(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	m.config = config
	globalConfig = config
	return nil
}

func (m *Manager) setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.readTimeout", "30s")
	viper.SetDefault("server.writeTimeout", "30s")
	viper.SetDefault("server.idleTimeout", "120s")
	viper.SetDefault("server.shutdownTimeout", "15s")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.dbname", "fern_platform")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.timezone", "UTC")
	viper.SetDefault("database.maxOpenConns", 25)
	viper.SetDefault("database.maxIdleConns", 5)
	viper.SetDefault("database.connMaxLifetime", "300s")
	viper.SetDefault("database.connMaxIdleTime", "300s")

	// Auth defaults
	viper.SetDefault("auth.enabled", false)
	viper.SetDefault("auth.tokenExpiry", "24h")
	viper.SetDefault("auth.refreshExpiry", "168h")

	// OAuth defaults
	viper.SetDefault("auth.oauth.enabled", false)
	viper.SetDefault("auth.oauth.scopes", []string{"openid", "profile", "email"})
	viper.SetDefault("auth.oauth.userIdField", "sub")
	viper.SetDefault("auth.oauth.emailField", "email")
	viper.SetDefault("auth.oauth.nameField", "name")
	viper.SetDefault("auth.oauth.groupsField", "groups")
	viper.SetDefault("auth.oauth.rolesField", "roles")

	// Role group name defaults
	viper.SetDefault("auth.oauth.adminGroupName", "admin")
	viper.SetDefault("auth.oauth.managerGroupName", "manager")
	viper.SetDefault("auth.oauth.userGroupName", "user")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.structured", true)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.poolSize", 10)
	viper.SetDefault("redis.idleTimeout", "300s")

	// LLM defaults
	viper.SetDefault("llm.defaultProvider", "anthropic")
	viper.SetDefault("llm.cacheEnabled", true)
	viper.SetDefault("llm.cacheTTL", "1h")
	viper.SetDefault("llm.maxTokens", 4000)
	viper.SetDefault("llm.temperature", 0.7)

	// Monitoring defaults
	viper.SetDefault("monitoring.metrics.enabled", true)
	viper.SetDefault("monitoring.metrics.path", "/metrics")
	viper.SetDefault("monitoring.metrics.port", 9090)
	viper.SetDefault("monitoring.health.path", "/health")
	viper.SetDefault("monitoring.health.interval", "30s")
	viper.SetDefault("monitoring.health.timeout", "5s")
}

func (m *Manager) bindEnvVars() error {
	// Server
	if err := viper.BindEnv("server.port", "PORT", "SERVER_PORT"); err != nil {
		return err
	}
	if err := viper.BindEnv("server.host", "HOST", "SERVER_HOST"); err != nil {
		return err
	}

	// Database
	if err := viper.BindEnv("database.host", "DB_HOST", "POSTGRES_HOST"); err != nil {
		return err
	}
	if err := viper.BindEnv("database.port", "DB_PORT", "POSTGRES_PORT"); err != nil {
		return err
	}
	if err := viper.BindEnv("database.user", "DB_USER", "POSTGRES_USER"); err != nil {
		return err
	}
	if err := viper.BindEnv("database.password", "DB_PASSWORD", "POSTGRES_PASSWORD"); err != nil {
		return err
	}
	if err := viper.BindEnv("database.dbname", "DB_NAME", "POSTGRES_DB"); err != nil {
		return err
	}
	if err := viper.BindEnv("database.sslmode", "DB_SSLMODE", "POSTGRES_SSLMODE"); err != nil {
		return err
	}

	// Auth
	if err := viper.BindEnv("auth.enabled", "AUTH_ENABLED"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.jwtSecret", "JWT_SECRET"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.jwksUrl", "JWKS_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.issuer", "AUTH_ISSUER"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.audience", "AUTH_AUDIENCE"); err != nil {
		return err
	}

	// OAuth
	if err := viper.BindEnv("auth.oauth.enabled", "OAUTH_ENABLED"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.clientId", "OAUTH_CLIENT_ID"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.clientSecret", "OAUTH_CLIENT_SECRET"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.redirectUrl", "OAUTH_REDIRECT_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.authUrl", "OAUTH_AUTH_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.tokenUrl", "OAUTH_TOKEN_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.userInfoUrl", "OAUTH_USERINFO_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.jwksUrl", "OAUTH_JWKS_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.issuerUrl", "OAUTH_ISSUER_URL"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.logoutUrl", "OAUTH_LOGOUT_URL"); err != nil {
		return err
	}

	// OAuth Admin and Field Mappings
	if err := viper.BindEnv("auth.oauth.adminUsers", "OAUTH_ADMIN_USERS"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.adminGroups", "OAUTH_ADMIN_GROUPS"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.userIdField", "OAUTH_USER_ID_FIELD"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.emailField", "OAUTH_EMAIL_FIELD"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.nameField", "OAUTH_NAME_FIELD"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.groupsField", "OAUTH_GROUPS_FIELD"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.rolesField", "OAUTH_ROLES_FIELD"); err != nil {
		return err
	}

	// Role group names
	if err := viper.BindEnv("auth.oauth.adminGroupName", "OAUTH_ADMIN_GROUP_NAME"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.managerGroupName", "OAUTH_MANAGER_GROUP_NAME"); err != nil {
		return err
	}
	if err := viper.BindEnv("auth.oauth.userGroupName", "OAUTH_USER_GROUP_NAME"); err != nil {
		return err
	}

	// Redis
	if err := viper.BindEnv("redis.host", "REDIS_HOST"); err != nil {
		return err
	}
	if err := viper.BindEnv("redis.port", "REDIS_PORT"); err != nil {
		return err
	}
	if err := viper.BindEnv("redis.password", "REDIS_PASSWORD"); err != nil {
		return err
	}

	// LLM Providers
	if err := viper.BindEnv("llm.providers.anthropic.apiKey", "ANTHROPIC_API_KEY"); err != nil {
		return err
	}
	if err := viper.BindEnv("llm.providers.openai.apiKey", "OPENAI_API_KEY"); err != nil {
		return err
	}
	if err := viper.BindEnv("llm.providers.huggingface.apiKey", "HUGGINGFACE_API_KEY"); err != nil {
		return err
	}

	// Logging
	if err := viper.BindEnv("logging.level", "LOG_LEVEL"); err != nil {
		return err
	}
	if err := viper.BindEnv("logging.format", "LOG_FORMAT"); err != nil {
		return err
	}
	
	return nil
}

func (m *Manager) validate(config *Config) error {
	// Database validation
	if config.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if config.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if config.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	// Auth validation
	if config.Auth.Enabled {
		if config.Auth.OAuth.Enabled {
			// OAuth validation
			if config.Auth.OAuth.ClientID == "" {
				return fmt.Errorf("oauth is enabled but client ID is missing")
			}
			if config.Auth.OAuth.ClientSecret == "" {
				return fmt.Errorf("oauth is enabled but client secret is missing")
			}
			if config.Auth.OAuth.AuthURL == "" {
				return fmt.Errorf("oauth is enabled but auth URL is missing")
			}
			if config.Auth.OAuth.TokenURL == "" {
				return fmt.Errorf("oauth is enabled but token URL is missing")
			}
			if config.Auth.OAuth.RedirectURL == "" {
				return fmt.Errorf("oauth is enabled but redirect URL is missing")
			}
		} else {
			// JWT validation for non-OAuth auth
			if config.Auth.JWTSecret == "" && config.Auth.JWKSUrl == "" {
				return fmt.Errorf("auth is enabled but no JWT secret or JWKS URL provided")
			}
		}
	}

	return nil
}

// GetConfig returns the global configuration instance
func GetConfig() *Config {
	if globalConfig == nil {
		panic("configuration not initialized - call config.Load() first")
	}
	return globalConfig
}

// GetDatabaseConfig returns database configuration
func GetDatabaseConfig() *DatabaseConfig {
	return &GetConfig().Database
}

// GetServerConfig returns server configuration
func GetServerConfig() *ServerConfig {
	return &GetConfig().Server
}

// GetAuthConfig returns authentication configuration
func GetAuthConfig() *AuthConfig {
	return &GetConfig().Auth
}

// GetServicesConfig returns services configuration
func GetServicesConfig() *ServicesConfig {
	return &GetConfig().Services
}

// GetLLMConfig returns LLM configuration
func GetLLMConfig() *LLMConfig {
	return &GetConfig().LLM
}

// DatabaseConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s.fern-platform:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

// MigrationURL returns the PostgreSQL URL for migrations
func (d *DatabaseConfig) MigrationURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s.fern-platform:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

// RedisConnectionString returns the Redis connection string
func (r *RedisConfig) ConnectionString() string {
	if r.Password != "" {
		return fmt.Sprintf("redis://:%s@%s:%d/%d", r.Password, r.Host, r.Port, r.DB)
	}
	return fmt.Sprintf("redis://%s:%d/%d", r.Host, r.Port, r.DB)
}
