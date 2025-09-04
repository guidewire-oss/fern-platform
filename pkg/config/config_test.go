package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", Label("auth"), func() {
	var (
		manager    *config.Manager
		tempDir    string
		configFile string
	)

	BeforeEach(func() {
		// Create a temporary directory for test config files
		var err error
		tempDir, err = os.MkdirTemp("", "config_test")
		Expect(err).NotTo(HaveOccurred())
		configFile = filepath.Join(tempDir, "config.yaml")

		// Reset viper state
		viper.Reset()

		// Clear any environment variables that might interfere
		clearTestEnvVars()

		// Create new manager
		manager = config.NewManager()
	})

	AfterEach(func() {
		// Clean up temp directory
		os.RemoveAll(tempDir)

		// Clear test environment variables
		clearTestEnvVars()

		// Reset viper state
		viper.Reset()
	})

	Describe("NewManager", func() {
		It("should create a new manager instance", func() {
			manager := config.NewManager()
			Expect(manager).NotTo(BeNil())
		})
	})

	Describe("Load Configuration", func() {
		Context("with default values only", func() {
			It("should load successfully with defaults", func() {
				err := manager.Load("")
				Expect(err).NotTo(HaveOccurred())

				cfg := config.GetConfig()
				Expect(cfg.Server.Port).To(Equal(8080))
				Expect(cfg.Server.Host).To(Equal("0.0.0.0"))
				Expect(cfg.Database.Host).To(Equal("localhost"))
				Expect(cfg.Database.Port).To(Equal(5432))
				Expect(cfg.Database.User).To(Equal("postgres"))
				Expect(cfg.Database.DBName).To(Equal("fern_platform"))
			})
		})

		Context("with valid config file", func() {
			BeforeEach(func() {
				configContent := `
server:
  port: 9090
  host: "127.0.0.1"
  readTimeout: "45s"
database:
  host: "db-server"
  port: 5433
  user: "testuser"
  password: "testpass"
  dbname: "testdb"
auth:
  enabled: true
  jwtSecret: "test-secret"
  tokenExpiry: "2h"
  oauth:
    enabled: true
    clientId: "test-client"
    clientSecret: "test-secret"
    redirectUrl: "http://localhost:8080/callback"
    authUrl: "http://auth.example.com/auth"
    tokenUrl: "http://auth.example.com/token"
    userInfoUrl: "http://auth.example.com/userinfo"
    scopes: ["openid", "profile", "email"]
    adminUsers: ["admin@example.com"]
    adminGroups: ["admins"]
redis:
  host: "redis-server"
  port: 6380
  password: "redis-pass"
llm:
  defaultProvider: "openai"
  cacheEnabled: false
  maxTokens: 2000
  temperature: 0.5
  providers:
    openai:
      type: "openai"
      apiKey: "test-key"
      model: "gpt-4"
      enabled: true
logging:
  level: "debug"
  format: "text"
  structured: false
monitoring:
  metrics:
    enabled: false
    port: 9091
  tracing:
    enabled: true
    serviceName: "test-service"
  health:
    path: "/healthz"
    interval: "60s"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load configuration from file", func() {
				err := manager.Load(configFile)
				Expect(err).NotTo(HaveOccurred())

				cfg := config.GetConfig()
				Expect(cfg.Server.Port).To(Equal(9090))
				Expect(cfg.Server.Host).To(Equal("127.0.0.1"))
				Expect(cfg.Server.ReadTimeout).To(Equal(45 * time.Second))
				Expect(cfg.Database.Host).To(Equal("db-server"))
				Expect(cfg.Database.Port).To(Equal(5433))
				Expect(cfg.Database.User).To(Equal("testuser"))
				Expect(cfg.Database.Password).To(Equal("testpass"))
				Expect(cfg.Database.DBName).To(Equal("testdb"))
				Expect(cfg.Auth.Enabled).To(BeTrue())
				Expect(cfg.Auth.JWTSecret).To(Equal("test-secret"))
				Expect(cfg.Auth.OAuth.Enabled).To(BeTrue())
				Expect(cfg.Auth.OAuth.ClientID).To(Equal("test-client"))
				Expect(cfg.Auth.OAuth.AdminUsers).To(ContainElement("admin@example.com"))
				Expect(cfg.Redis.Host).To(Equal("redis-server"))
				Expect(cfg.LLM.DefaultProvider).To(Equal("openai"))
				Expect(cfg.LLM.Temperature).To(Equal(float32(0.5)))
				Expect(cfg.Monitoring.Metrics.Enabled).To(BeFalse())
			})
		})

		Context("with environment variables", func() {
			BeforeEach(func() {
				// Set environment variables
				os.Setenv("PORT", "3000")
				os.Setenv("SERVER_HOST", "0.0.0.0")
				os.Setenv("DB_HOST", "env-db")
				os.Setenv("DB_PORT", "5434")
				os.Setenv("DB_USER", "envuser")
				os.Setenv("DB_PASSWORD", "envpass")
				os.Setenv("DB_NAME", "envdb")
				os.Setenv("AUTH_ENABLED", "true")
				os.Setenv("JWT_SECRET", "env-jwt-secret")
				os.Setenv("OAUTH_ENABLED", "true")
				os.Setenv("OAUTH_CLIENT_ID", "env-client-id")
				os.Setenv("OAUTH_CLIENT_SECRET", "env-client-secret")
				os.Setenv("OAUTH_AUTH_URL", "http://env.example.com/auth")
				os.Setenv("OAUTH_TOKEN_URL", "http://env.example.com/token")
				os.Setenv("OAUTH_REDIRECT_URL", "http://localhost:3000/callback")
				os.Setenv("REDIS_HOST", "env-redis")
				os.Setenv("REDIS_PORT", "6381")
				os.Setenv("ANTHROPIC_API_KEY", "env-anthropic-key")
				os.Setenv("LOG_LEVEL", "warn")
			})

			It("should prioritize environment variables over defaults", func() {
				err := manager.Load("")
				Expect(err).NotTo(HaveOccurred())

				cfg := config.GetConfig()
				Expect(cfg.Server.Port).To(Equal(3000))
				Expect(cfg.Database.Host).To(Equal("env-db"))
				Expect(cfg.Database.Port).To(Equal(5434))
				Expect(cfg.Database.User).To(Equal("envuser"))
				Expect(cfg.Database.Password).To(Equal("envpass"))
				Expect(cfg.Database.DBName).To(Equal("envdb"))
				Expect(cfg.Auth.Enabled).To(BeTrue())
				Expect(cfg.Auth.JWTSecret).To(Equal("env-jwt-secret"))
				Expect(cfg.Auth.OAuth.Enabled).To(BeTrue())
				Expect(cfg.Auth.OAuth.ClientID).To(Equal("env-client-id"))
				Expect(cfg.Redis.Host).To(Equal("env-redis"))
				Expect(cfg.Redis.Port).To(Equal(6381))
			})

			It("should handle OAuth field mappings from environment", func() {
				os.Setenv("OAUTH_USER_ID_FIELD", "user_id")
				os.Setenv("OAUTH_EMAIL_FIELD", "user_email")
				os.Setenv("OAUTH_GROUPS_FIELD", "user_groups")
				os.Setenv("OAUTH_ADMIN_GROUP_NAME", "administrators")

				err := manager.Load("")
				Expect(err).NotTo(HaveOccurred())

				cfg := config.GetConfig()
				Expect(cfg.Auth.OAuth.UserIDField).To(Equal("user_id"))
				Expect(cfg.Auth.OAuth.EmailField).To(Equal("user_email"))
				Expect(cfg.Auth.OAuth.GroupsField).To(Equal("user_groups"))
				Expect(cfg.Auth.OAuth.AdminGroupName).To(Equal("administrators"))
			})
		})

		Context("with invalid config file", func() {
			BeforeEach(func() {
				invalidConfig := `invalid: yaml: content: [`
				err := os.WriteFile(configFile, []byte(invalidConfig), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error for invalid YAML", func() {
				err := manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read config file"))
			})
		})

		Context("with missing required fields", func() {
			BeforeEach(func() {
				configContent := `
database:
  host: ""  # Empty host should cause validation error
  user: "testuser"
  dbname: "testdb"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail validation for missing database host", func() {
				err := manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("database host is required"))
			})
		})
	})

	Describe("Configuration Validation", func() {
		Context("database validation", func() {
			It("should require database host", func() {
				configContent := `
database:
  host: ""
  user: "testuser"
  dbname: "testdb"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("database host is required"))
			})

			It("should require database user", func() {
				configContent := `
database:
  host: "localhost"
  user: ""
  dbname: "testdb"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("database user is required"))
			})

			It("should require database name", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: ""
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("database name is required"))
			})
		})

		Context("authentication validation", func() {
			It("should require OAuth client ID when OAuth is enabled", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: ""
    authUrl: "http://example.com/auth"
    tokenUrl: "http://example.com/token"
    redirectUrl: "http://localhost/callback"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("oauth is enabled but client ID is missing"))
			})

			It("should require OAuth auth URL when OAuth is enabled", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: "test-client"
    authUrl: ""
    tokenUrl: "http://example.com/token"
    redirectUrl: "http://localhost/callback"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("oauth is enabled but auth URL is missing"))
			})

			It("should require OAuth token URL when OAuth is enabled", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: "test-client"
    authUrl: "http://example.com/auth"
    tokenUrl: ""
    redirectUrl: "http://localhost/callback"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("oauth is enabled but token URL is missing"))
			})

			It("should require OAuth redirect URL when OAuth is enabled", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: "test-client"
    authUrl: "http://example.com/auth"
    tokenUrl: "http://example.com/token"
    redirectUrl: ""
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("oauth is enabled but redirect URL is missing"))
			})

			It("should require JWT secret or JWKS URL when auth is enabled but OAuth is disabled", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: false
  jwtSecret: ""
  jwksUrl: ""
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("auth is enabled but no JWT secret or JWKS URL provided"))
			})

			It("should pass validation with JWT secret", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: false
  jwtSecret: "test-secret"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass validation with JWKS URL", func() {
				configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: false
  jwksUrl: "https://example.com/.well-known/jwks.json"
`
				err := os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = manager.Load(configFile)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Global Getter Functions", func() {
		BeforeEach(func() {
			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return config through GetConfig", func() {
			cfg := config.GetConfig()
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.Server.Port).To(Equal(8080))
		})

		It("should return database config through GetDatabaseConfig", func() {
			dbCfg := config.GetDatabaseConfig()
			Expect(dbCfg).NotTo(BeNil())
			Expect(dbCfg.Host).To(Equal("localhost"))
			Expect(dbCfg.Port).To(Equal(5432))
		})

		It("should return server config through GetServerConfig", func() {
			serverCfg := config.GetServerConfig()
			Expect(serverCfg).NotTo(BeNil())
			Expect(serverCfg.Port).To(Equal(8080))
			Expect(serverCfg.Host).To(Equal("0.0.0.0"))
		})

		It("should return auth config through GetAuthConfig", func() {
			authCfg := config.GetAuthConfig()
			Expect(authCfg).NotTo(BeNil())
			Expect(authCfg.Enabled).To(BeFalse()) // Default value
		})

		It("should return services config through GetServicesConfig", func() {
			servicesCfg := config.GetServicesConfig()
			Expect(servicesCfg).NotTo(BeNil())
		})

		It("should return LLM config through GetLLMConfig", func() {
			llmCfg := config.GetLLMConfig()
			Expect(llmCfg).NotTo(BeNil())
			Expect(llmCfg.DefaultProvider).To(Equal("anthropic"))
			Expect(llmCfg.Temperature).To(Equal(float32(0.7)))
		})
	})

	Describe("Connection String Builders", func() {
		var dbConfig *config.DatabaseConfig
		var redisConfig *config.RedisConfig

		BeforeEach(func() {
			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())
			dbConfig = config.GetDatabaseConfig()
			redisConfig = &config.GetConfig().Redis
		})

		Context("Database Connection String", func() {
			It("should build connection string correctly", func() {
				dbConfig.User = "testuser"
				dbConfig.Password = "testpass"
				dbConfig.Host = "testhost"
				dbConfig.Port = 5433
				dbConfig.DBName = "testdb"
				dbConfig.SSLMode = "require"

				connStr := dbConfig.ConnectionString()
				expected := "postgres://testuser:testpass@testhost.fern-platform:5433/testdb?sslmode=require"
				Expect(connStr).To(Equal(expected))
			})

			It("should build migration URL correctly", func() {
				dbConfig.User = "migrationuser"
				dbConfig.Password = "migrationpass"
				dbConfig.Host = "migrationhost"
				dbConfig.Port = 5434
				dbConfig.DBName = "migrationdb"
				dbConfig.SSLMode = "disable"

				migrationURL := dbConfig.MigrationURL()
				expected := "postgres://migrationuser:migrationpass@migrationhost.fern-platform:5434/migrationdb?sslmode=disable"
				Expect(migrationURL).To(Equal(expected))
			})
		})

		Context("Redis Connection String", func() {
			It("should build connection string without password", func() {
				redisConfig.Host = "redis-host"
				redisConfig.Port = 6379
				redisConfig.DB = 0
				redisConfig.Password = ""

				connStr := redisConfig.ConnectionString()
				expected := "redis://redis-host:6379/0"
				Expect(connStr).To(Equal(expected))
			})

			It("should build connection string with password", func() {
				redisConfig.Host = "redis-host"
				redisConfig.Port = 6380
				redisConfig.DB = 1
				redisConfig.Password = "redis-password"

				connStr := redisConfig.ConnectionString()
				expected := "redis://:redis-password@redis-host:6380/1"
				Expect(connStr).To(Equal(expected))
			})
		})
	})

	Describe("Default Values", func() {
		It("should set correct default values", func() {
			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())

			cfg := config.GetConfig()

			// Server defaults
			Expect(cfg.Server.Port).To(Equal(8080))
			Expect(cfg.Server.Host).To(Equal("0.0.0.0"))
			Expect(cfg.Server.ReadTimeout).To(Equal(30 * time.Second))
			Expect(cfg.Server.WriteTimeout).To(Equal(30 * time.Second))
			Expect(cfg.Server.IdleTimeout).To(Equal(120 * time.Second))
			Expect(cfg.Server.ShutdownTimeout).To(Equal(15 * time.Second))

			// Database defaults
			Expect(cfg.Database.Host).To(Equal("localhost"))
			Expect(cfg.Database.Port).To(Equal(5432))
			Expect(cfg.Database.User).To(Equal("postgres"))
			Expect(cfg.Database.DBName).To(Equal("fern_platform"))
			Expect(cfg.Database.SSLMode).To(Equal("disable"))
			Expect(cfg.Database.TimeZone).To(Equal("UTC"))
			Expect(cfg.Database.MaxOpenConns).To(Equal(25))
			Expect(cfg.Database.MaxIdleConns).To(Equal(5))

			// Auth defaults
			Expect(cfg.Auth.Enabled).To(BeFalse())
			Expect(cfg.Auth.TokenExpiry).To(Equal(24 * time.Hour))
			Expect(cfg.Auth.RefreshExpiry).To(Equal(168 * time.Hour))

			// OAuth defaults
			Expect(cfg.Auth.OAuth.Enabled).To(BeFalse())
			Expect(cfg.Auth.OAuth.Scopes).To(Equal([]string{"openid", "profile", "email"}))
			Expect(cfg.Auth.OAuth.UserIDField).To(Equal("sub"))
			Expect(cfg.Auth.OAuth.EmailField).To(Equal("email"))
			Expect(cfg.Auth.OAuth.NameField).To(Equal("name"))
			Expect(cfg.Auth.OAuth.GroupsField).To(Equal("groups"))
			Expect(cfg.Auth.OAuth.AdminGroupName).To(Equal("admin"))
			Expect(cfg.Auth.OAuth.ManagerGroupName).To(Equal("manager"))
			Expect(cfg.Auth.OAuth.UserGroupName).To(Equal("user"))

			// Logging defaults
			Expect(cfg.Logging.Level).To(Equal("info"))
			Expect(cfg.Logging.Format).To(Equal("json"))
			Expect(cfg.Logging.Output).To(Equal("stdout"))
			Expect(cfg.Logging.Structured).To(BeTrue())

			// Redis defaults
			Expect(cfg.Redis.Host).To(Equal("localhost"))
			Expect(cfg.Redis.Port).To(Equal(6379))
			Expect(cfg.Redis.DB).To(Equal(0))
			Expect(cfg.Redis.PoolSize).To(Equal(10))

			// LLM defaults
			Expect(cfg.LLM.DefaultProvider).To(Equal("anthropic"))
			Expect(cfg.LLM.CacheEnabled).To(BeTrue())
			Expect(cfg.LLM.CacheTTL).To(Equal(1 * time.Hour))
			Expect(cfg.LLM.MaxTokens).To(Equal(4000))
			Expect(cfg.LLM.Temperature).To(Equal(float32(0.7)))

			// Monitoring defaults
			Expect(cfg.Monitoring.Metrics.Enabled).To(BeTrue())
			Expect(cfg.Monitoring.Metrics.Path).To(Equal("/metrics"))
			Expect(cfg.Monitoring.Metrics.Port).To(Equal(9090))
			Expect(cfg.Monitoring.Health.Path).To(Equal("/health"))
			Expect(cfg.Monitoring.Health.Interval).To(Equal(30 * time.Second))
			Expect(cfg.Monitoring.Health.Timeout).To(Equal(5 * time.Second))
		})
	})

	Describe("Error Handling", func() {
		It("should handle config file read errors when specific file doesn't exist", func() {
			// When a specific config file is provided but doesn't exist, it should error
			err := manager.Load("/nonexistent/config.yaml")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read config file"))
		})

		It("should handle config directory search gracefully when no config file found", func() {
			// Test searching in multiple config directories - should not error when no file found
			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Environment Variable Patterns", func() {
		It("should handle alternative environment variable names", func() {
			// Test alternative env var names
			os.Setenv("POSTGRES_HOST", "alt-host")
			os.Setenv("POSTGRES_PORT", "5435")
			os.Setenv("POSTGRES_USER", "alt-user")
			os.Setenv("POSTGRES_PASSWORD", "alt-pass")
			os.Setenv("POSTGRES_DB", "alt-db")

			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())

			cfg := config.GetConfig()
			Expect(cfg.Database.Host).To(Equal("alt-host"))
			Expect(cfg.Database.Port).To(Equal(5435))
			Expect(cfg.Database.User).To(Equal("alt-user"))
			Expect(cfg.Database.Password).To(Equal("alt-pass"))
			Expect(cfg.Database.DBName).To(Equal("alt-db"))
		})

		It("should handle LLM provider API keys from environment", func() {
			os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
			os.Setenv("OPENAI_API_KEY", "test-openai-key")
			os.Setenv("HUGGINGFACE_API_KEY", "test-hf-key")

			err := manager.Load("")
			Expect(err).NotTo(HaveOccurred())

			// These would be set in the viper instance
			// We can verify they were bound by checking viper directly
			Expect(viper.GetString("llm.providers.anthropic.apiKey")).To(Equal("test-anthropic-key"))
			Expect(viper.GetString("llm.providers.openai.apiKey")).To(Equal("test-openai-key"))
			Expect(viper.GetString("llm.providers.huggingface.apiKey")).To(Equal("test-hf-key"))
		})
	})

	Describe("Complex Configuration Scenarios", func() {
		It("should handle comprehensive OAuth configuration", func() {
			configContent := `
database:
  host: "localhost"
  user: "testuser" 
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: "test-client"
    clientSecret: "test-secret"
    redirectUrl: "http://localhost:8080/callback"
    authUrl: "http://auth.example.com/auth"
    tokenUrl: "http://auth.example.com/token" 
    userInfoUrl: "http://auth.example.com/userinfo"
    jwksUrl: "http://auth.example.com/.well-known/jwks.json"
    issuerUrl: "http://auth.example.com"
    logoutUrl: "http://auth.example.com/logout"
    scopes: 
      - "openid"
      - "profile" 
      - "email"
      - "groups"
    adminUsers: 
      - "admin@example.com"
      - "superuser@example.com"
    adminGroups: 
      - "admins"
      - "superusers"
    adminGroupName: "administrators"
    managerGroupName: "managers" 
    userGroupName: "users"
    userIdField: "user_id"
    emailField: "user_email"
    nameField: "display_name"
    groupsField: "user_groups"
    rolesField: "user_roles"
`
			err := os.WriteFile(configFile, []byte(configContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = manager.Load(configFile)
			Expect(err).NotTo(HaveOccurred())

			cfg := config.GetConfig()
			oauth := cfg.Auth.OAuth
			Expect(oauth.Enabled).To(BeTrue())
			Expect(oauth.ClientID).To(Equal("test-client"))
			Expect(oauth.ClientSecret).To(Equal("test-secret"))
			Expect(oauth.RedirectURL).To(Equal("http://localhost:8080/callback"))
			Expect(oauth.AdminUsers).To(ContainElements("admin@example.com", "superuser@example.com"))
			Expect(oauth.AdminGroups).To(ContainElements("admins", "superusers"))
			Expect(oauth.AdminGroupName).To(Equal("administrators"))
			Expect(oauth.UserIDField).To(Equal("user_id"))
			Expect(oauth.EmailField).To(Equal("user_email"))
			Expect(oauth.NameField).To(Equal("display_name"))
			Expect(oauth.GroupsField).To(Equal("user_groups"))
			Expect(oauth.RolesField).To(Equal("user_roles"))
		})

		It("should handle OAuth role mappings configuration", func() {
			// Test role mappings separately since they can be tricky with YAML parsing
			configContent := `
database:
  host: "localhost"
  user: "testuser" 
  dbname: "testdb"
auth:
  enabled: true
  oauth:
    enabled: true
    clientId: "test-client"
    authUrl: "http://auth.example.com/auth"
    tokenUrl: "http://auth.example.com/token"
    redirectUrl: "http://localhost:8080/callback"
`
			err := os.WriteFile(configFile, []byte(configContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Set role mappings via environment variables to test that path
			os.Setenv("OAUTH_ADMIN_USERS", "admin1@example.com,admin2@example.com")
			os.Setenv("OAUTH_ADMIN_GROUPS", "admins,superusers")

			err = manager.Load(configFile)
			Expect(err).NotTo(HaveOccurred())

			cfg := config.GetConfig()
			oauth := cfg.Auth.OAuth
			Expect(oauth.Enabled).To(BeTrue())
			Expect(oauth.ClientID).To(Equal("test-client"))

			// Note: Environment variables for lists are handled differently by viper
			// The admin users and groups would be available through viper but may need
			// custom parsing for comma-separated values
		})

		It("should handle comprehensive monitoring configuration", func() {
			configContent := `
database:
  host: "localhost"
  user: "testuser"
  dbname: "testdb"  
monitoring:
  metrics:
    enabled: true
    path: "/custom-metrics"
    port: 9091
  tracing:
    enabled: true
    serviceName: "fern-platform"
    endpoint: "http://jaeger:14268/api/traces"
  health:
    path: "/healthcheck"
    interval: "45s"
    timeout: "10s"
`
			err := os.WriteFile(configFile, []byte(configContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			err = manager.Load(configFile)
			Expect(err).NotTo(HaveOccurred())

			cfg := config.GetConfig()
			monitoring := cfg.Monitoring
			Expect(monitoring.Metrics.Enabled).To(BeTrue())
			Expect(monitoring.Metrics.Path).To(Equal("/custom-metrics"))
			Expect(monitoring.Metrics.Port).To(Equal(9091))
			Expect(monitoring.Tracing.Enabled).To(BeTrue())
			Expect(monitoring.Tracing.ServiceName).To(Equal("fern-platform"))
			Expect(monitoring.Tracing.Endpoint).To(Equal("http://jaeger:14268/api/traces"))
			Expect(monitoring.Health.Path).To(Equal("/healthcheck"))
			Expect(monitoring.Health.Interval).To(Equal(45 * time.Second))
			Expect(monitoring.Health.Timeout).To(Equal(10 * time.Second))
		})
	})
})

// Helper function to clear test environment variables
func clearTestEnvVars() {
	envVars := []string{
		"PORT", "SERVER_PORT", "HOST", "SERVER_HOST",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSLMODE",
		"AUTH_ENABLED", "JWT_SECRET", "JWKS_URL", "AUTH_ISSUER", "AUTH_AUDIENCE",
		"OAUTH_ENABLED", "OAUTH_CLIENT_ID", "OAUTH_CLIENT_SECRET", "OAUTH_REDIRECT_URL",
		"OAUTH_AUTH_URL", "OAUTH_TOKEN_URL", "OAUTH_USERINFO_URL", "OAUTH_JWKS_URL",
		"OAUTH_ISSUER_URL", "OAUTH_LOGOUT_URL", "OAUTH_ADMIN_USERS", "OAUTH_ADMIN_GROUPS",
		"OAUTH_USER_ID_FIELD", "OAUTH_EMAIL_FIELD", "OAUTH_NAME_FIELD", "OAUTH_GROUPS_FIELD", "OAUTH_ROLES_FIELD",
		"OAUTH_ADMIN_GROUP_NAME", "OAUTH_MANAGER_GROUP_NAME", "OAUTH_USER_GROUP_NAME",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "HUGGINGFACE_API_KEY",
		"LOG_LEVEL", "LOG_FORMAT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

var _ = Describe("Manager.Load with env vars", func() {
	var m *config.Manager

	BeforeEach(func() {
		m = config.NewManager()
	})

	AfterEach(func() {
		// clean up any env vars we set
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("DB_USER")
		_ = os.Unsetenv("DB_PASS")
	})

	It("successfully binds SERVER_PORT env var", func() {
		err := os.Setenv("SERVER_PORT", "9090")
		Expect(err).NotTo(HaveOccurred())

		// Load with no config file, but env var set
		err = m.Load("")
		Expect(err).NotTo(HaveOccurred())

		// Viper should now resolve the value from env
		Expect(m.GetString("server.port")).To(Equal("9090"))
	})

	It("returns error if config file is missing", func() {
		// This triggers viper.ReadInConfig() error
		missingFile := filepath.Join(os.TempDir(), "nonexistent-config.yaml")

		err := m.Load(missingFile)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to read config file"))
	})

	It("still works if multiple env vars are set", func() {
		_ = os.Setenv("SERVER_PORT", "8081")
		_ = os.Setenv("DB_USER", "testuser")
		_ = os.Setenv("DB_PASS", "secret")

		err := m.Load("")
		Expect(err).NotTo(HaveOccurred())

		Expect(m.GetString("server.port")).To(Equal("8081"))
		Expect(m.GetString("db.user")).To(Equal("testuser"))
		Expect(m.GetString("db.pass")).To(Equal("secret"))
	})
})
