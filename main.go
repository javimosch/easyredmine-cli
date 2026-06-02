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
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func handleIssue(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli issue <subcommand> [options]")
		fmt.Fprintln(os.Stderr, "Subcommands: show, comment, edit")
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Unknown issue subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func handleIssueShow(args []string) {
	id, flags := extractPositional(args)
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(flags)

	if id == "" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli issue show <id> [--json]")
		os.Exit(1)
	}
	cfg := loadConfig()
	issue := getIssue(cfg, id)

	if *jsonOut {
		b, _ := json.MarshalIndent(issue, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("Issue #%d\n", issue.Issue.ID)
	fmt.Printf("Subject:   %s\n", issue.Issue.Subject)
	fmt.Printf("Project:   %s\n", issue.Issue.Project.Name)
	fmt.Printf("Tracker:   %s\n", issue.Issue.Tracker.Name)
	fmt.Printf("Status:    %s\n", issue.Issue.Status.Name)
	fmt.Printf("Priority:  %s\n", issue.Issue.Priority.Name)
	fmt.Printf("Author:    %s\n", issue.Issue.Author.Name)
	fmt.Printf("Created:   %s\n", issue.Issue.CreatedOn)
	fmt.Printf("Updated:   %s\n", issue.Issue.UpdatedOn)
	fmt.Printf("\nDescription:\n%s\n", issue.Issue.Description)

	if len(issue.Issue.Journals) > 0 {
		fmt.Printf("\nComments (%d):\n", len(issue.Issue.Journals))
		for _, j := range issue.Issue.Journals {
			if strings.TrimSpace(j.Notes) == "" {
				continue
			}
			fmt.Printf("\n--- %s (%s) ---\n", j.User.Name, j.CreatedOn)
			fmt.Println(j.Notes)
		}
	}
}

func handleIssueComment(args []string) {
	id, flags := extractPositional(args)
	fs := flag.NewFlagSet("comment", flag.ExitOnError)
	text := fs.String("text", "", "Comment text")
	jsonOut := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(flags)

	if id == "" || *text == "" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli issue comment <id> --text \"<comment>\" [--json]")
		os.Exit(1)
	}
	cfg := loadConfig()

	req := UpdateRequest{Issue: UpdateIssue{Notes: *text}}
	updateIssue(cfg, id, req)

	if *jsonOut {
		fmt.Printf("{\"ok\":true,\"issue_id\":%s,\"action\":\"comment\"}\n", id)
	} else {
		fmt.Printf("Comment added to issue #%s\n", id)
	}
}

func handleIssueEdit(args []string) {
	id, flags := extractPositional(args)
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	desc := fs.String("description", "", "New description text")
	jsonOut := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(flags)

	if id == "" || *desc == "" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli issue edit <id> --description \"<text>\" [--json]")
		os.Exit(1)
	}
	cfg := loadConfig()

	req := UpdateRequest{Issue: UpdateIssue{Description: *desc}}
	updateIssue(cfg, id, req)

	if *jsonOut {
		fmt.Printf("{\"ok\":true,\"issue_id\":%s,\"action\":\"edit_description\"}\n", id)
	} else {
		fmt.Printf("Issue #%s description updated\n", id)
	}
}

func handleConfig(args []string) {
	if len(args) == 0 {
		path := configPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "No config found. Use: easyredmine-cli config set")
			os.Exit(1)
		}
		cfg := loadConfig()
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(b))
		return
	}

	fs := flag.NewFlagSet("config", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() < 1 || fs.Arg(0) != "set" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli config set")
		os.Exit(1)
	}

	fmt.Print("EasyRedmine URL [https://easyredmine.simpliciti.fr]: ")
	var baseURL string
	fmt.Scanln(&baseURL)
	if baseURL == "" {
		baseURL = "https://easyredmine.simpliciti.fr"
	}

	fmt.Print("API Key: ")
	var apiKey string
	fmt.Scanln(&apiKey)
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "API Key is required")
		os.Exit(1)
	}

	cfg := Config{BaseURL: baseURL, APIKey: apiKey}
	saveConfig(cfg)
	fmt.Println("Config saved to", configPath())
}

// --- Redmine API ---

func getIssue(cfg Config, id string) IssueResponse {
	url := fmt.Sprintf("%s/issues/%s.json?include=journals", strings.TrimRight(cfg.BaseURL, "/"), id)
	body := doRequest(cfg, "GET", url, nil)

	var resp IssueResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("X-Redmine-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error making request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode > 299 {
		fmt.Fprintf(os.Stderr, "API error (%d): %s\n", resp.StatusCode, string(respBody))
		os.Exit(1)
	}

	return respBody
}

// --- Config ---

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ConfigDir, ConfigFile)
}

func loadConfig() Config {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No config found at %s\n", path)
		fmt.Fprintln(os.Stderr, "Run: easyredmine-cli config set")
		os.Exit(1)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func saveConfig(cfg Config) {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create config directory: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(configPath(), data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write config: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("easyredmine-cli - Redmine API client for EasyRedmine")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  easyredmine-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  issue        Manage Redmine issues")
	fmt.Println("  config       Manage CLI configuration")
	fmt.Println("  version      Show version")
	fmt.Println("  help         Show this help")
	fmt.Println()
	fmt.Println("Issue Subcommands:")
	fmt.Println("  show <id>          Show issue details")
	fmt.Println("  comment <id>       Add comment to issue")
	fmt.Println("    --text <text>      Comment text (required)")
	fmt.Println("  edit <id>          Edit issue fields")
	fmt.Println("    --description <t>  New description (required)")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  --json       Output in JSON format")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  easyredmine-cli issue show 61809")
	fmt.Println("  easyredmine-cli issue show 61809 --json")
	fmt.Println("  easyredmine-cli issue comment 61809 --text \"Looks good to me\"")
	fmt.Println("  easyredmine-cli issue edit 61809 --description \"Updated desc\"")
	fmt.Println("  easyredmine-cli config set")
}

// extractPositional splits args into the first non-flag token (id) and the rest (flags).
// This allows flags to appear before or after the positional id.
func extractPositional(args []string) (positional string, flags []string) {
	sawFlag := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
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
