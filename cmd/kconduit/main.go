package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/digitalis-io/kconduit/pkg/config"
	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/digitalis-io/kconduit/pkg/logger"
	"github.com/digitalis-io/kconduit/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgBrokers       string
	cfgLogLevel      string
	cfgLogFile       string
	cfgAiEngine      string
	cfgAiModel       string
	cfgSaslEnabled   bool
	cfgSaslMechanism string
	cfgSaslUsername  string
	cfgSaslPassword  string
	cfgSaslProtocol  string
	cfgTlsEnabled    bool
	cfgTlsCACert     string
	cfgTlsClientCert string
	cfgTlsClientKey  string
	cfgTlsSkipVerify bool
	cfgSession       string
	cfgSaveSession   string
)

// These variables are set via ldflags during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kconduit",
		Short: "Kconduit TUI for Kafka",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --list-sessions
			if cmd.Flags().Changed("list-sessions") {
				names, err := config.List()
				if err != nil {
					return fmt.Errorf("failed to list sessions: %w", err)
				}
				if len(names) == 0 {
					fmt.Println("No saved sessions.")
					return nil
				}
				for _, n := range names {
					fmt.Println(n)
				}
				return nil
			}

			// Apply saved session defaults before reading flags (CLI flags take precedence)
			if cfgSession != "" {
				sess, err := config.Load(cfgSession)
				if err != nil {
					return fmt.Errorf("failed to load session %q: %w", cfgSession, err)
				}
				applySessionDefaults(cmd, sess)
			}

			// If no --brokers or --session flag provided, run the startup session picker
			var activeSession string
			if !cmd.Flags().Changed("brokers") && cfgSession == "" && cfgSaveSession == "" && !cmd.Flags().Changed("list-sessions") {
				sc, err := ui.RunSessionPicker()
				if err != nil {
					return fmt.Errorf("session picker error: %w", err)
				}
				if sc == nil {
					// User quit the picker
					return nil
				}
				// Apply session values as Viper defaults (CLI flags still take precedence)
				applySessionDefaults(cmd, &sc.Session)
				if sc.Password != "" {
					viper.SetDefault("sasl_password", sc.Password)
				}
				activeSession = sc.Name
			} else {
				activeSession = cfgSession
			}

			// Merge Viper and flags
			brokers := viper.GetString("brokers")
			logLevel := viper.GetString("log_level")
			logFile := viper.GetString("log_file")
			aiEngine := viper.GetString("ai_engine")
			aiModel := viper.GetString("ai_model")
			saslEnabled := viper.GetBool("sasl_enabled")
			saslMechanism := viper.GetString("sasl_mechanism")
			saslUsername := viper.GetString("sasl_username")
			saslPassword := viper.GetString("sasl_password")
			saslProtocol := viper.GetString("sasl_protocol")
			tlsEnabled := viper.GetBool("tls_enabled")
			tlsCACert := viper.GetString("tls_ca_cert")
			tlsClientCert := viper.GetString("tls_client_cert")
			tlsClientKey := viper.GetString("tls_client_key")
			tlsSkipVerify := viper.GetBool("tls_skip_verify")

			// Handle --save-session: persist current config and exit
			if cfgSaveSession != "" {
				if cfgSaslPassword != "" {
					fmt.Fprintln(os.Stderr, "WARNING: passwords are not saved in session config; use KCONDUIT_SASL_PASSWORD env var")
				}
				sess := config.Session{
					Brokers:  brokers,
					LogLevel: logLevel,
					AIEngine: aiEngine,
					AIModel:  aiModel,
					SASL: config.SessionSASL{
						Enabled:   saslEnabled,
						Mechanism: saslMechanism,
						Username:  saslUsername,
						Protocol:  saslProtocol,
					},
					TLS: config.SessionTLS{
						Enabled:    tlsEnabled,
						CACert:     tlsCACert,
						ClientCert: tlsClientCert,
						ClientKey:  tlsClientKey,
						SkipVerify: tlsSkipVerify,
					},
				}
				if err := config.Save(cfgSaveSession, sess); err != nil {
					return fmt.Errorf("failed to save session: %w", err)
				}
				fmt.Printf("Session %q saved to %s\n", cfgSaveSession, config.ConfigPath())
				return nil
			}

			// Initialize logger
			if err := logger.Init(logLevel, logFile); err != nil {
				return fmt.Errorf("failed to initialize logger: %v", err)
			}

			// Parse brokers list
			brokerList := strings.Split(brokers, ",")
			for i := range brokerList {
				brokerList[i] = strings.TrimSpace(brokerList[i])
			}

			// Create SASL config if authentication is enabled
			var saslConfig *kafka.SASLConfig
			if saslEnabled {
				saslConfig = &kafka.SASLConfig{
					Enabled:   true,
					Mechanism: saslMechanism,
					Username:  saslUsername,
					Password:  saslPassword,
					Protocol:  saslProtocol,
				}
			}

			// Create TLS config if SSL is enabled or SASL_SSL is used
			var tlsConfig *kafka.TLSConfig
			if tlsEnabled || (saslConfig != nil && saslProtocol == "SASL_SSL") {
				tlsConfig = &kafka.TLSConfig{
					Enabled:            true,
					CACert:             tlsCACert,
					ClientCert:         tlsClientCert,
					ClientKey:          tlsClientKey,
					InsecureSkipVerify: tlsSkipVerify,
				}
			}

			// Kafka client with optional SASL authentication and TLS
			client, err := kafka.NewClientWithAuth(brokerList, saslConfig, tlsConfig)
			if err != nil {
				return fmt.Errorf("failed to connect to Kafka: %v", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					log.Printf("Error closing Kafka client: %v", err)
				}
			}()

			// Run UI
			model := ui.NewModel(client, aiEngine, aiModel, activeSession)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("error running program: %v", err)
			}

			return nil
		},
	}

	// Set version using Cobra's built-in version support
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("kconduit version %s\n  Build Time: %s\n  Git Commit: %s\n", Version, BuildTime, GitCommit))

	// Define flags
	rootCmd.Flags().StringVarP(&cfgBrokers, "brokers", "b", "localhost:9092", "Comma-separated list of Kafka broker addresses")
	rootCmd.Flags().StringVar(&cfgLogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().StringVar(&cfgLogFile, "log-file", "", "Log file path (if empty, logs to stderr)")
	rootCmd.Flags().StringVar(&cfgAiEngine, "ai-engine", "gemini", "AI engine to use (e.g., openai)")
	rootCmd.Flags().StringVar(&cfgAiModel, "ai-model", "gemini-flash-latest", "AI model to use (e.g., gpt-3.5-turbo, gpt-4)")

	// Session management flags
	rootCmd.Flags().StringVar(&cfgSession, "session", "", "Load a saved connection session by name")
	rootCmd.Flags().StringVar(&cfgSaveSession, "save-session", "", "Save current connection config as a named session and exit")
	rootCmd.Flags().Bool("list-sessions", false, "List all saved connection sessions and exit")

	// SASL authentication flags
	rootCmd.Flags().BoolVar(&cfgSaslEnabled, "sasl", false, "Enable SASL authentication")
	rootCmd.Flags().StringVar(&cfgSaslMechanism, "sasl-mechanism", "PLAIN", "SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)")
	rootCmd.Flags().StringVar(&cfgSaslUsername, "sasl-username", "", "SASL username")
	rootCmd.Flags().StringVar(&cfgSaslPassword, "sasl-password", "", "SASL password (deprecated: use KCONDUIT_SASL_PASSWORD env var instead)")
	_ = rootCmd.Flags().MarkDeprecated("sasl-password", "use the KCONDUIT_SASL_PASSWORD environment variable instead")
	rootCmd.Flags().StringVar(&cfgSaslProtocol, "sasl-protocol", "SASL_PLAINTEXT", "Security protocol (SASL_PLAINTEXT, SASL_SSL)")

	// TLS/SSL flags
	rootCmd.Flags().BoolVar(&cfgTlsEnabled, "tls", false, "Enable TLS/SSL")
	rootCmd.Flags().StringVar(&cfgTlsCACert, "tls-ca-cert", "", "Path to CA certificate file")
	rootCmd.Flags().StringVar(&cfgTlsClientCert, "tls-client-cert", "", "Path to client certificate file")
	rootCmd.Flags().StringVar(&cfgTlsClientKey, "tls-client-key", "", "Path to client key file")
	rootCmd.Flags().BoolVar(&cfgTlsSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification (insecure)")

	// Bind Viper to flags
	_ = viper.BindPFlag("brokers", rootCmd.Flags().Lookup("brokers"))
	_ = viper.BindPFlag("log_level", rootCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("log_file", rootCmd.Flags().Lookup("log-file"))
	_ = viper.BindPFlag("ai_engine", rootCmd.Flags().Lookup("ai-engine"))
	_ = viper.BindPFlag("ai_model", rootCmd.Flags().Lookup("ai-model"))
	_ = viper.BindPFlag("sasl_enabled", rootCmd.Flags().Lookup("sasl"))
	_ = viper.BindPFlag("sasl_mechanism", rootCmd.Flags().Lookup("sasl-mechanism"))
	_ = viper.BindPFlag("sasl_username", rootCmd.Flags().Lookup("sasl-username"))
	_ = viper.BindPFlag("sasl_password", rootCmd.Flags().Lookup("sasl-password"))
	_ = viper.BindPFlag("sasl_protocol", rootCmd.Flags().Lookup("sasl-protocol"))
	_ = viper.BindPFlag("tls_enabled", rootCmd.Flags().Lookup("tls"))
	_ = viper.BindPFlag("tls_ca_cert", rootCmd.Flags().Lookup("tls-ca-cert"))
	_ = viper.BindPFlag("tls_client_cert", rootCmd.Flags().Lookup("tls-client-cert"))
	_ = viper.BindPFlag("tls_client_key", rootCmd.Flags().Lookup("tls-client-key"))
	_ = viper.BindPFlag("tls_skip_verify", rootCmd.Flags().Lookup("tls-skip-verify"))

	// Environment variable support
	viper.SetEnvPrefix("KCONDUIT") // e.g. KCONDUIT_BROKERS
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// applySessionDefaults sets Viper defaults from a loaded session.
// Only applies values that were not explicitly set via CLI flags.
func applySessionDefaults(cmd *cobra.Command, sess *config.Session) {
	if sess.Brokers != "" && !cmd.Flags().Changed("brokers") {
		viper.SetDefault("brokers", sess.Brokers)
	}
	if sess.LogLevel != "" && !cmd.Flags().Changed("log-level") {
		viper.SetDefault("log_level", sess.LogLevel)
	}
	if sess.AIEngine != "" && !cmd.Flags().Changed("ai-engine") {
		viper.SetDefault("ai_engine", sess.AIEngine)
	}
	if sess.AIModel != "" && !cmd.Flags().Changed("ai-model") {
		viper.SetDefault("ai_model", sess.AIModel)
	}
	if sess.SASL.Enabled && !cmd.Flags().Changed("sasl") {
		viper.SetDefault("sasl_enabled", true)
		if sess.SASL.Mechanism != "" && !cmd.Flags().Changed("sasl-mechanism") {
			viper.SetDefault("sasl_mechanism", sess.SASL.Mechanism)
		}
		if sess.SASL.Username != "" && !cmd.Flags().Changed("sasl-username") {
			viper.SetDefault("sasl_username", sess.SASL.Username)
		}
		if sess.SASL.Protocol != "" && !cmd.Flags().Changed("sasl-protocol") {
			viper.SetDefault("sasl_protocol", sess.SASL.Protocol)
		}
		if sess.SASL.PasswordFile != "" && !cmd.Flags().Changed("sasl-password") {
			if pw, err := os.ReadFile(sess.SASL.PasswordFile); err == nil {
				viper.SetDefault("sasl_password", strings.TrimSpace(string(pw)))
			}
		}
	}
	if sess.TLS.Enabled && !cmd.Flags().Changed("tls") {
		viper.SetDefault("tls_enabled", true)
		if sess.TLS.CACert != "" && !cmd.Flags().Changed("tls-ca-cert") {
			viper.SetDefault("tls_ca_cert", sess.TLS.CACert)
		}
		if sess.TLS.ClientCert != "" && !cmd.Flags().Changed("tls-client-cert") {
			viper.SetDefault("tls_client_cert", sess.TLS.ClientCert)
		}
		if sess.TLS.ClientKey != "" && !cmd.Flags().Changed("tls-client-key") {
			viper.SetDefault("tls_client_key", sess.TLS.ClientKey)
		}
		if sess.TLS.SkipVerify && !cmd.Flags().Changed("tls-skip-verify") {
			viper.SetDefault("tls_skip_verify", true)
		}
	}
}
