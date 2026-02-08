package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/arcslash/ugudu/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Ugudu configuration",
		Long: `Manage Ugudu configuration including API keys and settings.

Configuration is stored in ~/.ugudu/config.yaml

Examples:
  ugudu config init              # Create config with wizard
  ugudu config show              # Show current config
  ugudu config set anthropic.api_key sk-ant-xxx
  ugudu config path              # Show config file path`,
	}

	cmd.AddCommand(configInitCmd())
	cmd.AddCommand(configShowCmd())
	cmd.AddCommand(configSetCmd())
	cmd.AddCommand(configPathCmd())

	return cmd
}

func configInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration with wizard",
		Long: `Create the Ugudu configuration file interactively.

This will guide you through setting up:
- API keys for AI providers
- Default provider and model
- Daemon settings`,
		Run: func(cmd *cobra.Command, args []string) {
			// Check if config exists
			if _, err := os.Stat(config.ConfigPath()); err == nil && !force {
				fmt.Printf("Config already exists at %s\n", config.ConfigPath())
				fmt.Println("Use --force to overwrite, or 'ugudu config set' to modify values.")
				return
			}

			// Ensure directories exist
			if err := config.EnsureDirectories(); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
				os.Exit(1)
			}

			reader := bufio.NewReader(os.Stdin)

			fmt.Println("╔══════════════════════════════════════════╗")
			fmt.Println("║       U G U D U   C O N F I G            ║")
			fmt.Println("║      Configuration Wizard                ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			fmt.Println()
			fmt.Printf("Config will be saved to: %s\n", config.ConfigPath())
			fmt.Println()
			fmt.Println("Enter API keys (press Enter to skip):")
			fmt.Println()

			cfg := config.DefaultConfig()

			// Anthropic
			fmt.Print("Anthropic API key: ")
			key, _ := reader.ReadString('\n')
			key = strings.TrimSpace(key)
			if key != "" {
				cfg.Providers.Anthropic.APIKey = key
			}

			// OpenAI
			fmt.Print("OpenAI API key: ")
			key, _ = reader.ReadString('\n')
			key = strings.TrimSpace(key)
			if key != "" {
				cfg.Providers.OpenAI.APIKey = key
			}

			// Groq
			fmt.Print("Groq API key: ")
			key, _ = reader.ReadString('\n')
			key = strings.TrimSpace(key)
			if key != "" {
				cfg.Providers.Groq.APIKey = key
			}

			// Ollama
			fmt.Print("Ollama URL (default: http://localhost:11434): ")
			url, _ := reader.ReadString('\n')
			url = strings.TrimSpace(url)
			if url != "" {
				cfg.Providers.Ollama.URL = url
			}

			fmt.Println()

			// Default provider
			fmt.Println("Default AI provider:")
			fmt.Println("  1) anthropic")
			fmt.Println("  2) openai")
			fmt.Println("  3) ollama")
			fmt.Println("  4) groq")
			fmt.Print("Choose [1-4] (default: 1): ")
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			switch choice {
			case "2":
				cfg.Defaults.Provider = "openai"
				cfg.Defaults.Model = "gpt-4o"
			case "3":
				cfg.Defaults.Provider = "ollama"
				cfg.Defaults.Model = "llama3.2"
			case "4":
				cfg.Defaults.Provider = "groq"
				cfg.Defaults.Model = "llama-3.1-70b-versatile"
			default:
				cfg.Defaults.Provider = "anthropic"
				cfg.Defaults.Model = "claude-sonnet-4-20250514"
			}

			fmt.Println()

			// Daemon port
			fmt.Print("Daemon HTTP port (default: 8080): ")
			port, _ := reader.ReadString('\n')
			port = strings.TrimSpace(port)
			if port != "" {
				cfg.Daemon.TCPAddr = ":" + port
			} else {
				cfg.Daemon.TCPAddr = ":8080"
			}

			// Save config
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			fmt.Println()
			fmt.Printf("Configuration saved to %s\n", config.ConfigPath())
			fmt.Println()
			fmt.Println("Directory structure created:")
			fmt.Printf("  %s/\n", config.UguduHome())
			fmt.Println("  ├── config.yaml    # Your configuration")
			fmt.Println("  ├── specs/         # Spec YAML files (blueprints)")
			fmt.Println("  └── data/          # Database")
			fmt.Println()
			fmt.Println("Next: Start the daemon with 'ugudu daemon'")
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing config")

	return cmd
}

func configShowCmd() *cobra.Command {
	var showSecrets bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			// Mask secrets unless --show-secrets
			if !showSecrets {
				if cfg.Providers.Anthropic.APIKey != "" {
					cfg.Providers.Anthropic.APIKey = maskKey(cfg.Providers.Anthropic.APIKey)
				}
				if cfg.Providers.OpenAI.APIKey != "" {
					cfg.Providers.OpenAI.APIKey = maskKey(cfg.Providers.OpenAI.APIKey)
				}
				if cfg.Providers.Groq.APIKey != "" {
					cfg.Providers.Groq.APIKey = maskKey(cfg.Providers.Groq.APIKey)
				}
			}

			data, _ := yaml.Marshal(cfg)
			fmt.Printf("# Config: %s\n\n", config.ConfigPath())
			fmt.Println(string(data))
		},
	}

	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "show API keys in full")

	return cmd
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Keys:
  anthropic.api_key    Anthropic API key
  openai.api_key       OpenAI API key
  groq.api_key         Groq API key
  ollama.url           Ollama URL
  defaults.provider    Default AI provider
  defaults.model       Default model
  daemon.tcp_addr      Daemon HTTP address

Examples:
  ugudu config set anthropic.api_key sk-ant-xxxxx
  ugudu config set defaults.provider openai
  ugudu config set daemon.tcp_addr :3000`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			// Ensure config directory exists
			if err := config.EnsureDirectories(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			switch key {
			case "anthropic.api_key":
				cfg.Providers.Anthropic.APIKey = value
			case "openai.api_key":
				cfg.Providers.OpenAI.APIKey = value
			case "groq.api_key":
				cfg.Providers.Groq.APIKey = value
			case "ollama.url":
				cfg.Providers.Ollama.URL = value
			case "defaults.provider":
				cfg.Defaults.Provider = value
			case "defaults.model":
				cfg.Defaults.Model = value
			case "daemon.tcp_addr":
				cfg.Daemon.TCPAddr = value
			default:
				fmt.Fprintf(os.Stderr, "Unknown key: %s\n", key)
				fmt.Println("\nAvailable keys:")
				fmt.Println("  anthropic.api_key, openai.api_key, groq.api_key")
				fmt.Println("  ollama.url, defaults.provider, defaults.model")
				fmt.Println("  daemon.tcp_addr")
				os.Exit(1)
			}

			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Set %s = %s\n", key, maskIfSecret(key, value))
		},
	}
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration paths",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Ugudu Home:  %s\n", config.UguduHome())
			fmt.Printf("Config:      %s\n", config.ConfigPath())
			fmt.Printf("Specs:       %s\n", config.SpecsDir())
			fmt.Printf("Data:        %s\n", config.DataDir())
			fmt.Printf("Socket:      %s\n", config.SocketPath())
		},
	}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func maskIfSecret(key, value string) string {
	if strings.Contains(key, "api_key") || strings.Contains(key, "secret") {
		return maskKey(value)
	}
	return value
}
