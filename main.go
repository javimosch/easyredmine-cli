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
	"time"
)

const Version = "1.0.3"
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
	AssignedTo  IDName    `json:"assigned_to"`
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
	Notes        string `json:"notes,omitempty"`
	Description  string `json:"description,omitempty"`
	StatusID     int    `json:"status_id,omitempty"`
	AssignedToID int    `json:"assigned_to_id,omitempty"`
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

type IssueListResponse struct {
	Issues     []IssueListItem `json:"issues"`
	TotalCount int             `json:"total_count"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
}

type IssueListItem struct {
	ID        int    `json:"id"`
	Project   IDName `json:"project"`
	Tracker   IDName `json:"tracker"`
	Status    IDName `json:"status"`
	Priority  IDName `json:"priority"`
	Author    IDName `json:"author"`
	Subject   string `json:"subject"`
	CreatedOn string `json:"created_on"`
	UpdatedOn string `json:"updated_on"`
}

type SearchResult struct {
	Results  []SearchHit `json:"results"`
	Total    int         `json:"total"`
	Returned int         `json:"returned"`
	Query    string      `json:"query"`
	Words    []string    `json:"words"`
}

type SearchHit struct {
	ID           int      `json:"id"`
	Subject      string   `json:"subject"`
	Project      IDName   `json:"project"`
	Status       IDName   `json:"status"`
	Priority     IDName   `json:"priority"`
	MatchCount   int      `json:"match_count"`
	MatchedWords []string `json:"matched_words"`
	UpdatedOn    string   `json:"updated_on"`
}

type UserListResponse struct {
	Users     []User `json:"users"`
	TotalCount int   `json:"total_count"`
	Offset     int   `json:"offset"`
	Limit      int   `json:"limit"`
}

type User struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Mail      string `json:"mail"`
	CreatedOn string `json:"created_on"`
}

type MembershipListResponse struct {
	Memberships []Membership `json:"memberships"`
	TotalCount  int          `json:"total_count"`
	Offset      int          `json:"offset"`
	Limit       int          `json:"limit"`
}

type Membership struct {
	ID      int         `json:"id"`
	Project IDName      `json:"project"`
	User    IDName      `json:"user"`
	Group   IDName      `json:"group"`
	Roles   []IDName    `json:"roles"`
}

type UserSearchResult struct {
	Results  []UserHit `json:"results"`
	Total    int       `json:"total"`
	Returned int       `json:"returned"`
	Query    string    `json:"query"`
}

type UserHit struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Fullname string `json:"fullname"`
	Mail     string `json:"mail"`
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
	case "user":
		handleUser(os.Args[2:])
	case "config":
		handleConfig(os.Args[2:])
	case "update":
		handleUpdate(os.Args[2:])
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
		fmt.Fprintln(os.Stderr, "Subcommands: search, show, comment, edit, status, assign")
		os.Exit(85)
	}

	sub := args[0]
	switch sub {
	case "search":
		handleIssueSearch(args[1:])
	case "show":
		handleIssueShow(args[1:])
	case "comment":
		handleIssueComment(args[1:])
	case "edit":
		handleIssueEdit(args[1:])
	case "status":
		handleIssueStatus(args[1:])
	case "assign":
		handleIssueAssign(args[1:])
	default:
		exitErr(85, "invalid_argument", fmt.Sprintf("Unknown issue subcommand: %s", sub), false, []string{"Run: easyredmine-cli help"})
	}
}

// --- smart search ---

var stopWords = map[string]bool{
	"le": true, "la": true, "les": true, "de": true, "des": true, "du": true,
	"un": true, "une": true, "dans": true, "pour": true, "sur": true, "par": true,
	"avec": true, "est": true, "sont": true, "pas": true, "non": true, "et": true,
	"ou": true, "mais": true, "que": true, "qui": true, "dont": true, "au": true,
	"aux": true, "ce": true, "ces": true, "cet": true, "cette": true, "se": true,
	"sa": true, "son": true, "ses": true, "lui": true, "elle": true, "ils": true,
	"elles": true, "nous": true, "vous": true, "en": true, "y": true,
	"ne": true, "plus": true, "très": true, "tout": true, "tous": true, "toute": true,
	"toutes": true, "chaque": true, "quelque": true, "quelques": true, "peut": true,
	"peuvent": true, "fait": true, "faire": true, "être": true, "avoir": true,
	"the": true, "a": true, "an": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "of": true, "by": true, "with": true, "from": true,
	"as": true, "but": true, "or": true, "if": true, "so": true,
	"no": true, "not": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "can": true,
	"could": true, "should": true, "may": true, "might": true, "shall": true,
	"this": true, "that": true, "these": true, "those": true, "it": true, "its": true,
	"i": true, "you": true, "he": true, "she": true, "we": true, "they": true,
	"me": true, "him": true, "us": true, "them": true, "my": true,
	"your": true, "his": true, "our": true, "their": true,
}

func tokenize(query string) []string {
	raw := strings.Fields(strings.ToLower(query))
	words := make([]string, 0, len(raw))
	for _, w := range raw {
		// strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}<>«»")
		if w == "" || stopWords[w] {
			continue
		}
		words = append(words, w)
	}
	return words
}

func fetchIssuePage(cfg Config, status, updatedOn string, offset, limit int) ([]IssueListItem, error) {
	url := fmt.Sprintf("%s/issues.json?status_id=%s&limit=%d&offset=%d&sort=id:asc",
		strings.TrimRight(cfg.BaseURL, "/"), status, limit, offset)
	if updatedOn != "" {
		url += fmt.Sprintf("&updated_on=>=%s", updatedOn)
	}
	body := doRequest(cfg, "GET", url, nil)

	var resp IssueListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return resp.Issues, nil
}

// wordMatch returns true if any word in the slice appears in the haystack (case-insensitive).
func wordMatch(haystack string, words []string) []string {
	lower := strings.ToLower(haystack)
	var matched []string
	for _, w := range words {
		if strings.Contains(lower, w) {
			matched = append(matched, w)
		}
	}
	return matched
}

func handleIssueSearch(args []string) {
	query, remaining := extractPositional(args)
	if query == "" {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue search \"<phrase>\" [options]", false, nil)
	}

	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Max results (default 20)")
	offset := fs.Int("offset", 0, "Result offset")
	status := fs.String("status", "open", "Status filter: open, *, or numeric ID")
	currentMonth := fs.Bool("current-month", false, "Only issues updated in current month")
	currentYear := fs.Bool("current-year", false, "Only issues updated in current year")
	after := fs.String("after", "", "Only issues updated after date (YYYY-MM-DD)")
	minMatches := fs.Int("min-matches", 1, "Minimum word matches to include")
	fs.Parse(remaining)

	// Build updated_on filter (API-side)
	now := time.Now()
	var updatedOnFilter string
	if *currentMonth {
		updatedOnFilter = fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
	} else if *currentYear {
		updatedOnFilter = fmt.Sprintf("%d-01-01", now.Year())
	} else if *after != "" {
		updatedOnFilter = *after
	}

	words := tokenize(query)
	if len(words) == 0 {
		exitErr(85, "invalid_argument", "No searchable words in query (all stop words filtered)", false, nil)
	}

	cfg := resolveConfig()

	// Step 1: count total open issues
	countURL := fmt.Sprintf("%s/issues.json?status_id=%s&limit=1", strings.TrimRight(cfg.BaseURL, "/"), *status)
	if updatedOnFilter != "" {
		countURL += fmt.Sprintf("&updated_on=>=%s", updatedOnFilter)
	}
	countBody := doRequest(cfg, "GET", countURL, nil)
	var countResp IssueListResponse
	if err := json.Unmarshal(countBody, &countResp); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error reading issue count: %v", err), false, nil)
	}
	totalIssues := countResp.TotalCount
	if totalIssues == 0 {
		result := SearchResult{Results: []SearchHit{}, Total: 0, Returned: 0, Query: query, Words: words}
		if human() {
			fmt.Println("No open issues to search")
		} else {
			outputJSON(result)
		}
		return
	}

	if !human() {
		fmt.Fprintf(os.Stderr, `{"progress":{"total":%d,"fetched":0,"status":"fetching"}}`+"\n", totalIssues)
	}

	// Step 2: fetch all pages concurrently
	pageSize := 100
	numPages := (totalIssues + pageSize - 1) / pageSize

	type pageResult struct {
		issues []IssueListItem
		err    error
	}
	ch := make(chan pageResult, numPages)

	concurrency := 20
	sem := make(chan struct{}, concurrency)

	for p := 0; p < numPages; p++ {
		go func(page int) {
			sem <- struct{}{}
			defer func() { <-sem }()
			off := page * pageSize
			issues, err := fetchIssuePage(cfg, *status, updatedOnFilter, off, pageSize)
			if err != nil {
				ch <- pageResult{err: err}
				return
			}
			ch <- pageResult{issues: issues}
		}(p)
	}

	// Collect all issues
	allIssues := make([]IssueListItem, 0, totalIssues)
	var lastErr error
	for p := 0; p < numPages; p++ {
		res := <-ch
		if res.err != nil {
			lastErr = res.err
			continue
		}
		allIssues = append(allIssues, res.issues...)
		if !human() {
			fmt.Fprintf(os.Stderr, `{"progress":{"total":%d,"fetched":%d,"status":"fetching"}}`+"\n", totalIssues, len(allIssues))
		}
	}
	close(ch)

	if lastErr != nil {
		exitErr(105, "integration_error", fmt.Sprintf("Error fetching issues: %v", lastErr), true, nil)
	}

	if !human() {
		fmt.Fprintf(os.Stderr, `{"progress":{"total":%d,"fetched":%d,"status":"filtering"}}`+"\n", totalIssues, len(allIssues))
	}

	// Step 3: client-side word matching
	type acc struct {
		hit          SearchHit
		matchCount   int
		matchedWords []string
	}
	collected := make(map[int]*acc)

	for _, item := range allIssues {
		// Match in subject
		matched := wordMatch(item.Subject, words)
		if len(matched) == 0 {
			continue
		}

		a := &acc{
			hit: SearchHit{
				ID:         item.ID,
				Subject:    item.Subject,
				Project:    item.Project,
				Status:     item.Status,
				Priority:   item.Priority,
				UpdatedOn:  item.UpdatedOn,
			},
			matchCount:   len(matched),
			matchedWords: matched,
		}
		collected[item.ID] = a
	}

	// Step 4: convert to slice
	hits := make([]SearchHit, 0, len(collected))
	for _, a := range collected {
		if a.matchCount >= *minMatches {
			a.hit.MatchCount = a.matchCount
			a.hit.MatchedWords = a.matchedWords
			hits = append(hits, a.hit)
		}
	}

	// Step 5: sort by match_count desc, then updated_on desc
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			swap := false
			if hits[j].MatchCount > hits[i].MatchCount {
				swap = true
			} else if hits[j].MatchCount == hits[i].MatchCount && hits[j].UpdatedOn > hits[i].UpdatedOn {
				swap = true
			}
			if swap {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}

	// Step 6: paginate
	total := len(hits)
	start := *offset
	if start > total {
		start = total
	}
	end := start + *limit
	if end > total {
		end = total
	}

	result := SearchResult{
		Results:  hits[start:end],
		Total:    total,
		Returned: end - start,
		Query:    query,
		Words:    words,
	}

	if !human() {
		fmt.Fprintf(os.Stderr, `{"progress":{"total":%d,"fetched":%d,"status":"done"}}`+"\n", totalIssues, len(allIssues))
	}

	if human() {
		if len(result.Results) == 0 {
			fmt.Println("No results")
			return
		}
		fmt.Printf("Query: %s\n", result.Query)
		fmt.Printf("Words: %s\n", strings.Join(result.Words, ", "))
		fmt.Printf("Results: %d/%d (showing %d)\n\n", result.Total, result.Total, result.Returned)
		for _, h := range result.Results {
			fmt.Printf("  #%d [%s] %s\n", h.ID, h.Status.Name, h.Subject)
			fmt.Printf("       Project: %s | Priority: %s | Matches: %d/%d\n",
				h.Project.Name, h.Priority.Name, h.MatchCount, len(result.Words))
		}
	} else {
		outputJSON(result)
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
		fmt.Printf("Subject:     %s\n", issue.Issue.Subject)
		fmt.Printf("Project:     %s\n", issue.Issue.Project.Name)
		fmt.Printf("Status:      %s\n", issue.Issue.Status.Name)
		fmt.Printf("Priority:    %s\n", issue.Issue.Priority.Name)
		fmt.Printf("Author:      %s\n", issue.Issue.Author.Name)
		if issue.Issue.AssignedTo.ID != 0 {
			fmt.Printf("Assigned to: %s\n", issue.Issue.AssignedTo.Name)
		}
		fmt.Printf("Updated:     %s\n", issue.Issue.UpdatedOn)
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

func handleIssueStatus(args []string) {
	id, remaining := extractPositional(args)
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	statusID := fs.Int("status-id", 0, "Status ID")
	fs.Parse(remaining)

	if id == "" || *statusID == 0 {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue status <id> --status-id <status_id>", false, nil)
	}
	cfg := resolveConfig()

	req := UpdateRequest{Issue: UpdateIssue{StatusID: *statusID}}
	updateIssue(cfg, id, req)

	if human() {
		fmt.Printf("Issue #%s status updated to ID %d\n", id, *statusID)
	} else {
		outputJSON(map[string]any{"ok": true, "issue_id": id, "action": "status_change", "status_id": *statusID})
	}
}

func handleIssueAssign(args []string) {
	id, remaining := extractPositional(args)
	fs := flag.NewFlagSet("assign", flag.ExitOnError)
	assignedToID := fs.Int("assigned-to-id", 0, "Assigned user ID")
	fs.Parse(remaining)

	if id == "" || *assignedToID == 0 {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli issue assign <id> --assigned-to-id <user_id>", false, nil)
	}
	cfg := resolveConfig()

	req := UpdateRequest{Issue: UpdateIssue{AssignedToID: *assignedToID}}
	updateIssue(cfg, id, req)

	if human() {
		fmt.Printf("Issue #%s assigned to user ID %d\n", id, *assignedToID)
	} else {
		outputJSON(map[string]any{"ok": true, "issue_id": id, "action": "assign", "assigned_to_id": *assignedToID})
	}
}

// --- user ---

func handleUser(args []string) {
	// Strip global --human/-H before subcommand dispatch
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--human" && a != "-H" {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: easyredmine-cli user <subcommand> [options]")
		fmt.Fprintln(os.Stderr, "Subcommands: search")
		os.Exit(85)
	}

	sub := args[0]
	switch sub {
	case "search":
		handleUserSearch(args[1:])
	default:
		exitErr(85, "invalid_argument", fmt.Sprintf("Unknown user subcommand: %s", sub), false, []string{"Run: easyredmine-cli help"})
	}
}

func handleUserSearch(args []string) {
	query, remaining := extractPositional(args)
	if query == "" {
		exitErr(85, "invalid_argument", "Usage: easyredmine-cli user search \"<name>\" --project-id <id>", false, nil)
	}

	fs := flag.NewFlagSet("user-search", flag.ExitOnError)
	projectID := fs.Int("project-id", 0, "Project ID (required)")
	limit := fs.Int("limit", 20, "Max results (default 20)")
	fs.Parse(remaining)

	if *projectID == 0 {
		exitErr(85, "invalid_argument", "Project ID is required. Use --project-id <id>", false, nil)
	}

	cfg := resolveConfig()

	// Fetch project memberships
	url := fmt.Sprintf("%s/projects/%d/memberships.json?limit=100", strings.TrimRight(cfg.BaseURL, "/"), *projectID)
	body := doRequest(cfg, "GET", url, nil)

	var resp MembershipListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error parsing membership list: %v", err), false, nil)
	}

	// Search for matching users (case-insensitive)
	queryLower := strings.ToLower(query)
	var hits []UserHit
	for _, membership := range resp.Memberships {
		// Check users
		if membership.User.ID != 0 && strings.Contains(strings.ToLower(membership.User.Name), queryLower) {
			hits = append(hits, UserHit{
				ID:       membership.User.ID,
				Login:    membership.User.Name,
				Fullname: membership.User.Name,
				Mail:     "",
			})
		}
		// Check groups
		if membership.Group.ID != 0 && strings.Contains(strings.ToLower(membership.Group.Name), queryLower) {
			hits = append(hits, UserHit{
				ID:       membership.Group.ID,
				Login:    membership.Group.Name,
				Fullname: membership.Group.Name + " (Group)",
				Mail:     "",
			})
		}
		// Check roles
		for _, role := range membership.Roles {
			if strings.Contains(strings.ToLower(role.Name), queryLower) {
				// Add the user/group with this role
				if membership.User.ID != 0 {
					hits = append(hits, UserHit{
						ID:       membership.User.ID,
						Login:    membership.User.Name,
						Fullname: membership.User.Name + " (" + role.Name + ")",
						Mail:     "",
					})
				} else if membership.Group.ID != 0 {
					hits = append(hits, UserHit{
						ID:       membership.Group.ID,
						Login:    membership.Group.Name,
						Fullname: membership.Group.Name + " (Group, " + role.Name + ")",
						Mail:     "",
					})
				}
			}
		}
	}

	// Limit results
	if len(hits) > *limit {
		hits = hits[:*limit]
	}

	result := UserSearchResult{
		Results:  hits,
		Total:    len(hits),
		Returned: len(hits),
		Query:    query,
	}

	if human() {
		if len(result.Results) == 0 {
			fmt.Println("No users found")
			return
		}
		fmt.Printf("Query: %s (Project ID: %d)\n", result.Query, *projectID)
		fmt.Printf("Results: %d\n\n", result.Returned)
		for _, h := range result.Results {
			fmt.Printf("  ID: %d | Name: %s\n", h.ID, h.Fullname)
		}
	} else {
		outputJSON(result)
	}
}

// --- update ---

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func handleUpdate(args []string) {
	// Strip global --human/-H before processing
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--human" && a != "-H" {
			filtered = append(filtered, a)
		}
	}
	args = filtered

	fs := flag.NewFlagSet("update", flag.ExitOnError)
	checkOnly := fs.Bool("check-only", false, "Only check for updates, don't install")
	fs.Parse(args)

	// Fetch latest release from GitHub
	url := "https://api.github.com/repos/javimosch/easyredmine-cli/releases/latest"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error creating request: %v", err), false, nil)
	}
	req.Header.Set("User-Agent", "easyredmine-cli")
	
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		exitErr(105, "integration_error", fmt.Sprintf("Failed to fetch release info: %v", err), true, nil)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			exitErr(92, "resource_not_found", "No releases found on GitHub. This is a new repository - releases will be available after the first official release.", false, []string{"To update manually: Clone the repository, build with 'go build -ldflags=\"-s -w\" -o easyredmine-cli main.go', and copy to your PATH"})
		}
		exitErr(105, "integration_error", fmt.Sprintf("GitHub API returned status %d", resp.StatusCode), true, nil)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error reading response: %v", err), false, nil)
	}
	
	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Error parsing release info: %v", err), false, nil)
	}
	
	// Parse version from tag (remove 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := Version
	
	if human() {
		fmt.Printf("Current version: %s\n", currentVersion)
		fmt.Printf("Latest version:  %s\n", latestVersion)
	}
	
	// Compare versions (simple string comparison for now)
	if latestVersion == currentVersion {
		if human() {
			fmt.Println("Already up to date!")
		} else {
			outputJSON(map[string]any{"ok": true, "current": currentVersion, "latest": latestVersion, "action": "no_update_needed"})
		}
		return
	}

	if *checkOnly {
		if human() {
			fmt.Printf("Update available: %s → %s\n", currentVersion, latestVersion)
			fmt.Println("Use --check-only=false to install the update.")
		} else {
			outputJSON(map[string]any{"ok": true, "current": currentVersion, "latest": latestVersion, "action": "update_available", "check_only": true})
		}
		return
	}
	
	if human() {
		fmt.Printf("Update available: %s → %s\n", currentVersion, latestVersion)
		fmt.Println("Downloading...")
	} else {
		outputJSON(map[string]any{"ok": true, "current": currentVersion, "latest": latestVersion, "action": "downloading"})
	}
	
	// Find the Linux binary asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "linux") && !strings.Contains(asset.Name, ".md5") && !strings.Contains(asset.Name, ".sha256") {
			downloadURL = asset.DownloadURL
			break
		}
	}
	
	if downloadURL == "" {
		exitErr(105, "integration_error", "No Linux binary found in release assets", true, nil)
	}
	
	// Download to /tmp
	tmpPath := "/tmp/easyredmine-cli-update"
	resp, err = http.Get(downloadURL)
	if err != nil {
		exitErr(105, "integration_error", fmt.Sprintf("Failed to download binary: %v", err), true, nil)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		exitErr(105, "integration_error", fmt.Sprintf("Download failed with status %d", resp.StatusCode), true, nil)
	}
	
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Failed to create temp file: %v", err), false, nil)
	}
	defer tmpFile.Close()
	
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Failed to write binary: %v", err), false, nil)
	}
	
	// Make it executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		exitErr(110, "internal_error", fmt.Sprintf("Failed to make binary executable: %v", err), false, nil)
	}
	
	// Determine install path
	installPath := "/usr/local/bin/easyredmine-cli"
	home, err := os.UserHomeDir()
	if err == nil {
		localBin := filepath.Join(home, ".local", "bin", "easyredmine-cli")
		if _, err := os.Stat(localBin); err == nil {
			installPath = localBin
		}
	}
	
	if human() {
		fmt.Printf("Installing to: %s\n", installPath)
	}
	
	// Replace the binary
	if err := os.Rename(tmpPath, installPath); err != nil {
		// Try copy if rename fails (different filesystems)
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			exitErr(110, "internal_error", fmt.Sprintf("Failed to read downloaded binary: %v", err), false, nil)
		}
		if err := os.WriteFile(installPath, data, 0755); err != nil {
			exitErr(110, "internal_error", fmt.Sprintf("Failed to install binary: %v", err), false, nil)
		}
		os.Remove(tmpPath)
	}
	
	if human() {
		fmt.Printf("Successfully updated to %s\n", latestVersion)
	} else {
		outputJSON(map[string]any{"ok": true, "current": currentVersion, "latest": latestVersion, "action": "updated", "install_path": installPath})
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

	client := &http.Client{Timeout: 15 * time.Second}
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
	fmt.Println("  issue search <phrase>  Smart search (word-by-word, dedup, rank by match)")
	fmt.Println("    --limit <n>            Max results (default 20)")
	fmt.Println("    --offset <n>           Result offset")
	fmt.Println("    --status <s>           Status filter: open, *, or numeric ID (default open)")
	fmt.Println("    --current-month        Only issues updated this month (API-side filter)")
	fmt.Println("    --current-year         Only issues updated this year (API-side filter)")
	fmt.Println("    --after <date>         Only issues updated after YYYY-MM-DD (API-side filter)")
	fmt.Println("    --min-matches <n>      Min word matches (default 1)")
	fmt.Println("  issue show <id>       Show issue details (JSON output by default)")
	fmt.Println("  issue comment <id>    Add comment to issue")
	fmt.Println("    --text <text>         Comment text (required)")
	fmt.Println("  issue edit <id>       Edit issue fields")
	fmt.Println("    --description <t>     New description (required)")
	fmt.Println("  issue status <id>     Change issue status")
	fmt.Println("    --status-id <n>       Status ID (required)")
	fmt.Println("  issue assign <id>     Assign issue to user/group")
	fmt.Println("    --assigned-to-id <n>  User/Group ID (required)")
	fmt.Println("  user search <query>    Search users/groups/roles in project")
	fmt.Println("    --project-id <n>      Project ID (required)")
	fmt.Println("  update                 Check for updates from GitHub")
	fmt.Println("    --check-only          Only check, don't install")
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
	fmt.Println("  easyredmine-cli issue search \"correction statut message\"")
	fmt.Println("  easyredmine-cli issue search \"correction statut\" --limit 50")
	fmt.Println("  easyredmine-cli issue search \"correction\" --current-month   # fast: 1 page")
	fmt.Println("  easyredmine-cli issue search \"correction\" --current-year    # ~8 pages")
	fmt.Println("  easyredmine-cli issue search \"correction\" --after 2026-05-01 # custom date")
	fmt.Println("  easyredmine-cli issue show 61809")
	fmt.Println("  easyredmine-cli issue show 61809 --human")
	fmt.Println("  easyredmine-cli issue comment 61809 --text \"Looks good\"")
	fmt.Println("  easyredmine-cli issue edit 61809 --description \"<p>Updated</p>\"")
	fmt.Println("  easyredmine-cli issue status 61809 --status-id 51")
	fmt.Println("  easyredmine-cli issue assign 61809 --assigned-to-id 199")
	fmt.Println("  easyredmine-cli user search \"QA\" --project-id 1111")
	fmt.Println("  easyredmine-cli update --check-only")
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
