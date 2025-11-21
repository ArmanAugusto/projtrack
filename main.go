package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
)

// Project represents a tracked project.
type Project struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
	DueDate   time.Time `json:"due_date"`
	Done      bool      `json:"done"`
}

// Storage file location: ~/.projtrack.json
func storagePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".projtrack.json"), nil
}

func loadProjects() ([]Project, error) {
	path, err := storagePath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist yet, return empty list
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Project{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func saveProjects(projects []Project) error {
	path, err := storagePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func nextID(projects []Project) int {
	maxID := 0
	for _, p := range projects {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	return maxID + 1
}

func parseDate(s string) (time.Time, error) {
	// Expect YYYY-MM-DD
	const layout = "2006-01-02"
	return time.Parse(layout, s)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// Determine color + status label based on due date and done flag.
func statusColorAndLabel(p Project, now time.Time) (string, string) {
	if p.Done {
		return ColorCyan, "DONE"
	}

	diff := p.DueDate.Sub(now)
	days := int(diff.Hours() / 24)

	switch {
	case days < 0:
		return ColorRed, fmt.Sprintf("OVERDUE (%d days ago)", -days)
	case days == 0:
		return ColorRed, "DUE TODAY"
	case days <= 2:
		return ColorRed, fmt.Sprintf("DUE IN %d DAYS", days)
	case days <= 7:
		return ColorYellow, fmt.Sprintf("DUE IN %d DAYS", days)
	default:
		return ColorGreen, fmt.Sprintf("DUE IN %d DAYS", days)
	}
}

func cmdAdd(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	name := fs.String("name", "", "Project name (required)")
	startStr := fs.String("start", "", "Start date YYYY-MM-DD (optional, defaults to today)")
	dueStr := fs.String("due", "", "Due date YYYY-MM-DD (required)")
	fs.Parse(args)

	if *name == "" {
		fmt.Println("Error: -name is required")
		fs.Usage()
		os.Exit(1)
	}
	if *dueStr == "" {
		fmt.Println("Error: -due is required")
		fs.Usage()
		os.Exit(1)
	}

	now := time.Now()

	var startDate time.Time
	var err error
	if *startStr == "" {
		startDate = now
	} else {
		startDate, err = parseDate(*startStr)
		if err != nil {
			fmt.Printf("Invalid start date: %v\n", err)
			os.Exit(1)
		}
	}

	dueDate, err := parseDate(*dueStr)
	if err != nil {
		fmt.Printf("Invalid due date: %v\n", err)
		os.Exit(1)
	}

	projects, err := loadProjects()
	if err != nil {
		fmt.Printf("Error loading projects: %v\n", err)
		os.Exit(1)
	}

	p := Project{
		ID:        nextID(projects),
		Name:      *name,
		StartDate: startDate,
		DueDate:   dueDate,
		Done:      false,
	}

	projects = append(projects, p)

	if err := saveProjects(projects); err != nil {
		fmt.Printf("Error saving projects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added project #%d: %s (start: %s, due: %s)\n",
		p.ID, p.Name, formatDate(p.StartDate), formatDate(p.DueDate))
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	showAll := fs.Bool("all", true, "Show all projects (true by default)")
	fs.Parse(args)

	projects, err := loadProjects()
	if err != nil {
		fmt.Printf("Error loading projects: %v\n", err)
		os.Exit(1)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found yet. Add one with `projtrack add`.")
		return
	}

	now := time.Now()

	// Sort by due date
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].DueDate.Before(projects[j].DueDate)
	})

	fmt.Println("ID  NAME                           START       DUE         STATUS")
	fmt.Println("-------------------------------------------------------------------")
	for _, p := range projects {
		if !*showAll && p.Done {
			continue
		}
		color, status := statusColorAndLabel(p, now)
		fmt.Printf("%-3d %-30s %-10s %-10s %s%-20s%s\n",
			p.ID,
			truncate(p.Name, 30),
			formatDate(p.StartDate),
			formatDate(p.DueDate),
			color,
			status,
			ColorReset,
		)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func cmdDone(args []string) {
	fs := flag.NewFlagSet("done", flag.ExitOnError)
	idStr := fs.String("id", "", "Project ID to mark as done (required)")
	fs.Parse(args)

	if *idStr == "" {
		fmt.Println("Error: -id is required")
		fs.Usage()
		os.Exit(1)
	}

	id, err := strconv.Atoi(*idStr)
	if err != nil {
		fmt.Printf("Invalid ID: %v\n", err)
		os.Exit(1)
	}

	projects, err := loadProjects()
	if err != nil {
		fmt.Printf("Error loading projects: %v\n", err)
		os.Exit(1)
	}

	found := false
	for i := range projects {
		if projects[i].ID == id {
			projects[i].Done = true
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("No project with ID %d\n", id)
		os.Exit(1)
	}

	if err := saveProjects(projects); err != nil {
		fmt.Printf("Error saving projects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Marked project #%d as done.\n", id)
}

func printUsage() {
	fmt.Println(`Usage:
  projtrack <command> [options]

Commands:
  add    Add a new project
  list   List projects
  done   Mark a project as done

Examples:
  projtrack add -name "FPGA Toolchain" -start 2025-11-21 -due 2025-12-10
  projtrack list
  projtrack done -id 1`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "add":
		cmdAdd(args)
	case "list":
		cmdList(args)
	case "done":
		cmdDone(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}
