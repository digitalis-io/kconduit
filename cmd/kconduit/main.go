package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/axonops/kconduit/pkg/logger"
	"github.com/axonops/kconduit/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Handle version flag
			if viper.GetBool("version") {
				fmt.Printf("kconduit version %s\n", Version)
				fmt.Printf("  Build Time: %s\n", BuildTime)
				fmt.Printf("  Git Commit: %s\n", GitCommit)
				os.Exit(0)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			// Version flag is handled before RunE, so this code path won't be reached
			// when --version is used

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

			// Kafka client with optional SASL authentication
			client, err := kafka.NewClientWithAuth(brokerList, saslConfig)
			if err != nil {
				return fmt.Errorf("failed to connect to Kafka: %v", err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					log.Printf("Error closing Kafka client: %v", err)
				}
			}()

			// Run UI
			model := ui.NewModel(client, aiEngine, aiModel)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("error running program: %v", err)
			}

			return nil
		},
	}

	// Define flags
	rootCmd.Flags().StringVarP(&cfgBrokers, "brokers", "b", "localhost:9092", "Comma-separated list of Kafka broker addresses")
	rootCmd.Flags().StringVar(&cfgLogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().StringVar(&cfgLogFile, "log-file", "", "Log file path (if empty, logs to stderr)")
	rootCmd.Flags().StringVar(&cfgAiEngine, "ai-engine", "gemini", "AI engine to use (e.g., openai)")
	rootCmd.Flags().StringVar(&cfgAiModel, "ai-model", "gemini-1.5-pro-latest", "AI model to use (e.g., gpt-3.5-turbo, gpt-4)")

	// SASL authentication flags
	rootCmd.Flags().BoolVar(&cfgSaslEnabled, "sasl", false, "Enable SASL authentication")
	rootCmd.Flags().StringVar(&cfgSaslMechanism, "sasl-mechanism", "PLAIN", "SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)")
	rootCmd.Flags().StringVar(&cfgSaslUsername, "sasl-username", "", "SASL username")
	rootCmd.Flags().StringVar(&cfgSaslPassword, "sasl-password", "", "SASL password")
	rootCmd.Flags().StringVar(&cfgSaslProtocol, "sasl-protocol", "SASL_PLAINTEXT", "Security protocol (SASL_PLAINTEXT, SASL_SSL)")

	// Version flag
	rootCmd.Flags().BoolP("version", "v", false, "Print version information and exit")

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
	_ = viper.BindPFlag("version", rootCmd.Flags().Lookup("version"))

	// Environment variable support
	viper.SetEnvPrefix("KCONDUIT") // e.g. KCONDUIT_BROKERS
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
