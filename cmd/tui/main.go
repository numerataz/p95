package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"sixtyseven/internal/tui"
	"sixtyseven/pkg/client"
)

// Config holds TUI configuration
type Config struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
	APIKey  string `json:"api_key,omitempty"`
}

func main() {
	// Load or create config
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Determine which credential to use (prefer API key over token)
	credential := cfg.APIKey
	if credential == "" {
		credential = cfg.Token
	}

	// If no credential, prompt for login
	if credential == "" {
		cfg, err = interactiveLogin(cfg)
		if err != nil {
			fmt.Printf("Login failed: %v\n", err)
			os.Exit(1)
		}
		if err := saveConfig(cfg); err != nil {
			fmt.Printf("Warning: Failed to save config: %v\n", err)
		}
		credential = cfg.Token
	}

	// Create API client
	apiClient := client.New(cfg.BaseURL, client.WithToken(credential))

	// Create and run TUI
	app := tui.New(apiClient)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sixtyseven/config.json"
	}
	return filepath.Join(home, ".sixtyseven", "config.json")
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		BaseURL: "http://localhost:8080",
	}

	// Check environment variables first
	if url := os.Getenv("SIXTYSEVEN_URL"); url != "" {
		cfg.BaseURL = url
	}
	if apiKey := os.Getenv("SIXTYSEVEN_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
		return cfg, nil
	}
	if token := os.Getenv("SIXTYSEVEN_TOKEN"); token != "" {
		cfg.Token = token
		return cfg, nil
	}

	// Try to load from file
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func saveConfig(cfg *Config) error {
	path := configPath()

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func interactiveLogin(cfg *Config) (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to Sixtyseven!")
	fmt.Println()

	// Server URL
	fmt.Printf("Server URL [%s]: ", cfg.BaseURL)
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	if url != "" {
		cfg.BaseURL = url
	}

	// Email
	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	// Password (hidden)
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	fmt.Println()

	// Create client and login
	apiClient := client.New(cfg.BaseURL)
	token, err := apiClient.Login(email, password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	cfg.Token = token
	fmt.Println("Login successful!")
	fmt.Println()

	return cfg, nil
}
