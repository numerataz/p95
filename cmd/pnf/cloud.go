package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultCloudURL = "https://p.ninetyfive.gg"

type cloudCredentials struct {
	APIKey      string `json:"api_key"`
	URL         string `json:"url"`
	DefaultTeam string `json:"default_team,omitempty"`
}

func defaultConfigDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "p95")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "p95")
	default: // Linux and others
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		return filepath.Join(xdgConfig, "p95")
	}
}

func credentialsPath() string {
	return filepath.Join(defaultConfigDir(), "credentials.json")
}

func loadCredentials() (*cloudCredentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var creds cloudCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func saveCredentials(creds cloudCredentials) error {
	dir := defaultConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credentialsPath(), data, 0600)
}

func cloudCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: pnf cloud <command>\n\nCommands:\n  login   Authenticate and store an API key\n  logout  Remove stored credentials\n  status  Show current authentication status")
		os.Exit(1)
	}
	switch args[0] {
	case "login":
		cloudLoginCmd(args[1:])
	case "logout":
		cloudLogoutCmd()
	case "status":
		cloudStatusCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown cloud command: %s\n", args[0])
		os.Exit(1)
	}
}

func cloudLogoutCmd() {
	path := credentialsPath()
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Not logged in.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error removing credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Logged out. Credentials removed.")
}

func cloudLoginCmd(args []string) {
	url := os.Getenv("P95_URL")
	if url == "" {
		url = defaultCloudURL
	}
	// Allow --url override
	for i, a := range args {
		if a == "--url" && i+1 < len(args) {
			url = args[i+1]
		} else if v, ok := strings.CutPrefix(a, "--url="); ok {
			url = v
		}
	}
	url = strings.TrimRight(url, "/")

	// Step 1: Open browser to API keys settings page
	apiKeysURL := url + "/settings/api-keys"
	fmt.Printf("Opening %s in your browser...\n", apiKeysURL)
	fmt.Println("Generate a new API key and paste it below.")
	fmt.Println()
	openURL(apiKeysURL)

	// Step 2: Read pasted API key
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("API key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "No API key provided.")
		os.Exit(1)
	}

	// Step 3: Validate key and get user info
	req, _ := http.NewRequest(http.MethodGet, url+"/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %v\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		fmt.Fprintln(os.Stderr, "Invalid API key. Please generate a valid key and try again.")
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error verifying API key (status %d)\n", resp.StatusCode)
		os.Exit(1)
	}

	var user struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading user info: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Fetch default (personal) team
	teamReq, _ := http.NewRequest(http.MethodGet, url+"/api/v1/teams", nil)
	teamReq.Header.Set("Authorization", "Bearer "+apiKey)
	teamResp, err := http.DefaultClient.Do(teamReq)
	defaultTeam := ""
	if err == nil && teamResp.StatusCode == http.StatusOK {
		defer teamResp.Body.Close()
		var teams []struct {
			Slug       string `json:"slug"`
			IsPersonal bool   `json:"is_personal"`
		}
		if json.NewDecoder(teamResp.Body).Decode(&teams) == nil {
			for _, t := range teams {
				if t.IsPersonal {
					defaultTeam = t.Slug
					break
				}
			}
			// Fall back to first team if no personal team found
			if defaultTeam == "" && len(teams) > 0 {
				defaultTeam = teams[0].Slug
			}
		}
	}

	// Step 5: Save credentials
	if err := saveCredentials(cloudCredentials{APIKey: apiKey, URL: url, DefaultTeam: defaultTeam}); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Logged in as %s.", user.Email)
	if defaultTeam != "" {
		fmt.Printf(" Default team: %s.", defaultTeam)
	}
	fmt.Printf("\nCredentials saved to %s\n", credentialsPath())
}

func cloudStatusCmd(_ []string) {
	// Env vars take priority
	apiKey := os.Getenv("P95_API_KEY")
	url := os.Getenv("P95_URL")
	source := "environment"
	defaultTeam := ""

	// Load credentials file (for default team and as fallback for key/url)
	creds, err := loadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading credentials: %v\n", err)
		os.Exit(1)
	}
	if creds != nil {
		defaultTeam = creds.DefaultTeam
		if apiKey == "" {
			apiKey = creds.APIKey
			url = creds.URL
			source = credentialsPath()
		}
	}

	if apiKey == "" {
		fmt.Println("No credentials found. Run 'pnf cloud login' to authenticate.")
		return
	}

	if url == "" {
		url = defaultCloudURL
	}

	// Verify the key with the API
	req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(url, "/")+"/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Linked to %s (key: %s...)\nCould not verify: %v\n", url, apiKey[:min(12, len(apiKey))], err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Printf("API key invalid or expired. Run 'pnf cloud login' to re-authenticate.\nSource: %s\n", source)
		return
	}

	var user struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	json.NewDecoder(resp.Body).Decode(&user)

	prefix := apiKey
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	fmt.Printf("Linked to %s as %s (key: %s...)\n", url, user.Email, prefix)
	if defaultTeam != "" {
		fmt.Printf("Default team: %s\n", defaultTeam)
	}
	fmt.Printf("Credentials from: %s\n", source)
}


