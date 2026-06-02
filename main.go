package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const Version = "1.0.0"
const ConfigDir = ".config/easyredmine-cli"
const ConfigFile = "config.json"

type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type IssueResponse struct {
	Issue Issue `json:"issue"`
}

type Issue struct {
	ID          int       `json:"id"`
	Project     IDName    `json:"project"`
	Tracker     IDName    `json:"tracker"`
	Status      IDName    `json:"status"`
	Priority    IDName    `json:"priority"`
	Author      IDName    `json:"author"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	CreatedOn   string    `json:"created_on"`
	UpdatedOn   string    `json:"updated_on"`
	Journals    []Journal `json:"journals"`
}

type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Journal struct {
	ID        int    `json:"id"`
	User      IDName `json:"user"`
	Notes     string `json:"notes"`
	CreatedOn string `json:"created_on"`
}

type UpdateRequest struct {
	Issue UpdateIssue `json:"issue"`
}

type UpdateIssue struct {
	Notes       string `json:"notes,omitempty"`
	Description string `json:"description,omitempty"`
}

type ErrorBody struct {
	Error ErrorInfo `json:"error"`
}

type ErrorInfo struct {
	Code        int      `json:"code"`
	Type        string   `json:"type"`
	Message     string   `json:"message"`
	Recoverable bool     `json:"recoverable"`
	Suggestions []string `json:"suggestions,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "issue":
		handleIssue(os.Args[2:])
	case "config":
		handleConfig(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("easyredmine-cli v%s\n", Version)
	case "help", "--help", "-h":
		printHelp()
	default:
		exitErr(85, "invalid_argument", fmt.Sprintf("Unknown command: %s", command), false, []string{"Run: easyredmine-cli help"})
	}
}

// --- helpers ---

func isHuman() bool {
	for _, a := range os.Args {
		if a == "--human" || a == "-H" {
			return true
		}
	}
	return false
}

func human() bool { return isHuman() }

func exitErr(code int, etype, msg string, recoverable bool, suggestions []string) {
	if human() {
		fmt.Fprintf(os.Stderr, "Error [%d/%s]: %s\n", code, etype, msg)
		for _, s := range suggestions {
			fmt.Fprintf(os.Stderr, "  Suggestion: %s\n", s)
		}
	} else {
		b, _ := json.Marshal(ErrorBody{
			Error: ErrorInfo{Code: code, Type: etype, Message: msg, Recoverable: recoverable, Suggestions: suggestions},
		})
		fmt.Fprintln(os.Stderr, string(b))
	}
	os.Exit(code)
}

func outputJSON(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

// --- issue ---

func handleIssue(args []string) {
	// Strip global --human/-H before subcommand dispatch
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--human" && a != "-H" {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli issue <subcommand> [options]")
		fmt.Fprintln(os.Stderr, "Subcommands: show, comment, edit")
		os.Exit(85)
	}

	sub := args[0]
	switch sub {
	case "show":
		handleIssueShow(args[1:])
	case "comment":
		handleIssueComment(args[1:])
	case "edit":
		handleIssueEdit(args[1:])
	default:
		exitErr(85, "invalid_argument", fmt.Sprintf("Unknown issue subcommand: %s", sub), false, []string{"Run: easyredmine-cli help"})
	}
}

func handleIssueShow(args []string) {
	id, remaining := extractPositional(args)
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	fs.Parse(remaining)

	if id == "" {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue show <id>", false, nil)
	}
	cfg := resolveConfig()
	issue := getIssue(cfg, id)

	if human() {
		fmt.Printf("Issue #%d\n", issue.Issue.ID)
		fmt.Printf("Subject:   %s\n", issue.Issue.Subject)
		fmt.Printf("Project:   %s\n", issue.Issue.Project.Name)
		fmt.Printf("Status:    %s\n", issue.Issue.Status.Name)
		fmt.Printf("Priority:  %s\n", issue.Issue.Priority.Name)
		fmt.Printf("Updated:   %s\n", issue.Issue.UpdatedOn)
		fmt.Printf("\nDescription:\n%s\n", issue.Issue.Description)
		for _, j := range issue.Issue.Journals {
			if strings.TrimSpace(j.Notes) == "" {
				continue
			}
			fmt.Printf("\n--- %s (%s) ---\n%s\n", j.User.Name, j.CreatedOn, j.Notes)
		}
	} else {
		outputJSON(issue)
	}
}

func handleIssueComment(args []string) {
	id, remaining := extractPositional(args)
	fs := flag.NewFlagSet("comment", flag.ExitOnError)
	text := fs.String("text", "", "Comment text")
	fs.Parse(remaining)

	if id == "" || *text == "" {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue comment <id> --text \"<text>\"", false, nil)
	}
	cfg := resolveConfig()

	req := UpdateRequest{Issue: UpdateIssue{Notes: *text}}
	updateIssue(cfg, id, req)

	if human() {
		fmt.Printf("Comment added to issue #%s\n", id)
	} else {
		outputJSON(map[string]any{"ok": true, "issue_id": id, "action": "comment"})
	}
}

func handleIssueEdit(args []string) {
	id, remaining := extractPositional(args)
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	desc := fs.String("description", "", "New description text")
	fs.Parse(remaining)

	if id == "" || *desc == "" {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue edit <id> --description \"<text>\"", false, nil)
	}
	cfg := resolveConfig()

	req := UpdateRequest{Issue: UpdateIssue{Description: *desc}}
	updateIssue(cfg, id, req)

	if human() {
		fmt.Printf("Issue #%s description updated\n", id)
	} else {
		outputJSON(map[string]any{"ok": true, "issue_id": id, "action": "edit_description"})
	}
}

// --- config ---

func handleConfig(args []string) {
	if len(args) == 0 {
		path := configPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			exitErr(92, "resource_not_found", "No config found", false, []string{"Run: easyredmine-cli config set --api-key <key>"})
		}
		cfg := loadConfigFile()
		outputJSON(cfg)
		return
	}

	// config set --api-key <key> [--base-url <url>]
	if len(args) >= 1 && args[0] == "set" {
		fs := flag.NewFlagSet("config-set", flag.ExitOnError)
		apiKey := fs.String("api-key", "", "EasyRedmine API key")
		baseURL := fs.String("base-url", "https://easyredmine.simpliciti.fr", "EasyRedmine base URL")
		fs.Parse(args[1:])

		key := *apiKey
		if key == "" {
			key = os.Getenv("EASYREDMINE_API_KEY")
		}
		if key == "" {
			exitErr(85, "invalid_argument", "API key required. Use --api-key <key> or EASYREDMINE_API_KEY env var", false, nil)
		}

		url := *baseURL
		if envURL := os.Getenv("EASYREDMINE_BASE_URL"); envURL != "" {
			url = envURL
		}

		cfg := Config{BaseURL: url, APIKey: key}
		saveConfigFile(cfg)
		if human() {
			fmt.Println("Config saved to", configPath())
		} else {
			outputJSON(map[string]any{"ok": true, "path": configPath()})
		}
		return
	}

	exitErr(85, "invalid_argument", "Usage: easyredmine-cli config set --api-key <key> [--base-url <url>]", false, nil)
}

// --- Redmine API ---

func getIssue(cfg Config, id string) IssueResponse {
	url := fmt.Sprintf("%s/issues/%s.json?include=journals", strings.TrimRight(cfg.BaseURL, "/"), id)
	body := doRequest(cfg, "GET", url, nil)

	var resp IssueResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error parsing response: %v", err), false, nil)
	}
	if resp.Issue.ID == 0 {
		exitErr(92, "resource_not_found", fmt.Sprintf("Issue %s not found", id), false, nil)
	}
	return resp
}

func updateIssue(cfg Config, id string, req UpdateRequest) {
	reqBody, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/issues/%s.json", strings.TrimRight(cfg.BaseURL, "/"), id)
	doRequest(cfg, "PUT", url, reqBody)
}

func doRequest(cfg Config, method, url string, body []byte) []byte {
	var reqBody io.Reader
	if body != nil {
		reqBody = strings.NewReader(string(body))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error creating request: %v", err), false, nil)
	}

	req.Header.Set("X-Redmine-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		exitErr(105, "integration_error", fmt.Sprintf("API request failed: %v", err), true, nil)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error reading response: %v", err), false, nil)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		exitErr(85, "authentication_error", "Invalid API key", false, []string{"Re-run: easyredmine-cli config set --api-key <key>"})
	}
	if resp.StatusCode == 404 {
		exitErr(92, "resource_not_found", "Issue not found", false, nil)
	}
	if resp.StatusCode > 299 {
		exitErr(105, "integration_error", fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(respBody)), true, nil)
	}

	return respBody
}

// --- Config ---

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Cannot find home directory: %v", err), false, nil)
	}
	return filepath.Join(home, ConfigDir, ConfigFile)
}

func resolveConfig() Config {
	cfg := Config{BaseURL: "https://easyredmine.simpliciti.fr"}

	// Env vars take precedence
	if key := os.Getenv("EASYREDMINE_API_KEY"); key != "" {
		cfg.APIKey = key
	}
	if url := os.Getenv("EASYREDMINE_BASE_URL"); url != "" {
		cfg.BaseURL = url
	}

	// Fall back to config file for missing fields
	fileCfg, err := readConfigFile()
	if err == nil {
		if cfg.APIKey == "" {
			cfg.APIKey = fileCfg.APIKey
		}
		if cfg.BaseURL == "https://easyredmine.simpliciti.fr" && fileCfg.BaseURL != "" {
			cfg.BaseURL = fileCfg.BaseURL
		}
	}

	if cfg.APIKey == "" {
		exitErr(85, "invalid_argument", "No API key configured. Use EASYREDMINE_API_KEY env var or run: easyredmine-cli config set --api-key <key>", false, nil)
	}

	return cfg
}

func readConfigFile() (Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadConfigFile() Config {
	cfg, err := readConfigFile()
	if err != nil {
		exitErr(92, "resource_not_found", fmt.Sprintf("No config found at %s", configPath()), false, []string{"Run: easyredmine-cli config set --api-key <key>"})
	}
	return cfg
}

func saveConfigFile(cfg Config) {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Cannot create config directory: %v", err), false, nil)
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configPath(), data, 0600); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Cannot write config: %v", err), false, nil)
	}
}

// --- help ---

func printHelp() {
	fmt.Println("easyredmine-cli v" + Version + " — Redmine API client for EasyRedmine")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  easyredmine-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  issue show <id>       Show issue details (JSON output by default)")
	fmt.Println("  issue comment <id>    Add comment to issue")
	fmt.Println("    --text <text>         Comment text (required)")
	fmt.Println("  issue edit <id>       Edit issue fields")
	fmt.Println("    --description <t>     New description (required)")
	fmt.Println("  config                 Show current config")
	fmt.Println("  config set             Set API key and base URL")
	fmt.Println("    --api-key <key>        EasyRedmine API key (or EASYREDMINE_API_KEY env)")
	fmt.Println("    --base-url <url>       EasyRedmine base URL (default: https://easyredmine.simpliciti.fr)")
	fmt.Println("  version                Show version")
	fmt.Println("  help                   Show this help")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  --human, -H  Human-readable output (default: JSON)")
	fmt.Println()
	fmt.Println("Env Vars:")
	fmt.Println("  EASYREDMINE_API_KEY    API key (overrides config file)")
	fmt.Println("  EASYREDMINE_BASE_URL   Base URL (overrides config file)")
	fmt.Println()
	fmt.Println("Exit Codes:")
	fmt.Println("  0    Success")
	fmt.Println("  85   Invalid argument / configuration error")
	fmt.Println("  92   Resource not found")
	fmt.Println("  105  Integration / API error")
	fmt.Println("  110  Internal error")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  easyredmine-cli issue show 61809")
	fmt.Println("  easyredmine-cli issue show 61809 --human")
	fmt.Println("  easyredmine-cli issue comment 61809 --text \"Looks good\"")
	fmt.Println("  easyredmine-cli issue edit 61809 --description \"<p>Updated</p>\"")
	fmt.Println("  easyredmine-cli config set --api-key <key>")
	fmt.Println("  EASYREDMINE_API_KEY=<key> easyredmine-cli issue show 61809")
}

// extractPositional splits args into the first non-flag token (id) and the rest (flags).
// Strips --human/-H so subcommand parsers don't error on the global flag.
func extractPositional(args []string) (positional string, flags []string) {
	sawFlag := false
	for _, arg := range args {
		if arg == "--human" || arg == "-H" {
			continue
		}
		if strings.HasPrefix(arg, "-") && !sawFlag {
			sawFlag = true
		}
		if sawFlag {
			flags = append(flags, arg)
		} else {
			positional = arg
		}
	}
	return
}
