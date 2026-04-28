package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-addon/ssh"
	hub "github.com/konveyor/tackle2-hub/addon"
)

var (
	addon     = hub.Addon
	Dir       = ""
	SourceDir = ""
	Source    = "Kai"

	PalletBin = "/usr/bin/pallet"
	GooseBin  = "/usr/bin/goose"
	SkillsDir = "/addon/skills"
)

// Data matches the TaskGroup data shape sent by the tackle2-ui migrate modal.
type Data struct {
	MigrationTarget string        `json:"migrationTarget,omitempty"`
	Pallet          *PalletConfig `json:"pallet,omitempty"`

	// Legacy: kept for compatibility with existing tackle2-addon repository fetch
	Repository repository.SCM `json:"repository,omitempty"`
	Source     string         `json:"source,omitempty"`
}

type PalletConfig struct {
	YAML      string   `json:"yaml,omitempty"`
	Archetype *Ref     `json:"archetype,omitempty"`
	Skills    []string `json:"skills,omitempty"`
}

type Ref struct {
	ID   uint   `json:"id"`
	Name string `json:"name,omitempty"`
}

func init() {
	Dir, _ = os.Getwd()
	SourceDir = path.Join(Dir, "source")
}

// env returns the value of the named environment variable, or fallback if unset.
func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	addon.Run(func() (err error) {
		d := &Data{}
		err = addon.DataWith(d)
		if err != nil {
			return
		}
		if d.Source == "" {
			d.Source = Source
		}

		// Agent config comes from the kai-config secret (env vars).
		agentName := env("KAI_AGENT", "goose")
		provider := os.Getenv("KAI_PROVIDER")
		model := os.Getenv("KAI_MODEL")

		//
		// Fetch application.
		addon.Activity("Fetching application.")
		application, err := addon.Task.Application()
		if err != nil {
			return
		}

		// SSH agent
		agent := ssh.Agent{}
		err = agent.Start()
		if err != nil {
			return
		}

		// Clone from application.Repository.
		addon.Activity("Cloning repository.")
		err = FetchRepository(application)
		if err != nil {
			return
		}

		// Write pallet.yaml into the workspace if the UI provided pallet config.
		if d.Pallet != nil && d.Pallet.YAML != "" {
			palletPath := path.Join(SourceDir, "pallet.yaml")
			err = os.WriteFile(palletPath, []byte(d.Pallet.YAML), 0644)
			if err != nil {
				return fmt.Errorf("writing pallet.yaml: %w", err)
			}
		}

		// Sync skills into the workspace via pallet.
		addon.Activity("Syncing skills.")
		err = PalletSync(SourceDir)
		if err != nil {
			addon.Activity("Pallet sync skipped: %v", err)
		}

		// Environment for the agent subprocess so fetch-analysis can reach the Hub.
		hubEnv := []string{
			fmt.Sprintf("HUB_BASE_URL=%s", os.Getenv("HUB_BASE_URL")),
			fmt.Sprintf("HUB_TOKEN=%s", os.Getenv("TOKEN")),
			fmt.Sprintf("APP_ID=%d", application.ID),
		}
		if d.MigrationTarget != "" {
			hubEnv = append(hubEnv, fmt.Sprintf("MIGRATION_TARGET=%s", d.MigrationTarget))
		}

		// Run the AI agent.
		addon.Activity("Running migration agent (%s).", agentName)
		skillFile := path.Join(SkillsDir, "migration", "SKILL.md")
		err = RunAgent(agentName, provider, model, hubEnv, skillFile)
		if err != nil {
			return
		}

		// Push results to source repo on a new branch.
		// Branch name: <migrator-task-name>-<task-id> to avoid conflicts.
		taskID := os.Getenv("TASK")
		branchName := fmt.Sprintf("konveyor-migration/%s-%s",
			sanitize(application.Name), taskID)
		addon.Activity("Pushing migration branch.")
		err = PushBranch(SourceDir, branchName)
		if err != nil {
			return
		}

		addon.Activity("Migration complete. Branch: %s", branchName)
		return
	})
}

// PalletSync runs `pallet sync .` in the workspace directory.
func PalletSync(workDir string) error {
	if _, err := os.Stat(PalletBin); os.IsNotExist(err) {
		return fmt.Errorf("pallet binary not found at %s", PalletBin)
	}
	cmd := exec.Command(PalletBin, "sync", ".")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunAgent executes goose or opencode with the migration skill.
func RunAgent(agentName, provider, model string, hubEnv []string, skillFile string) error {
	switch strings.ToLower(agentName) {
	case "opencode":
		return runOpenCode(model, hubEnv, skillFile)
	default:
		return runGoose(provider, model, hubEnv, skillFile)
	}
}

func runGoose(provider, model string, hubEnv []string, skillFile string) error {
	args := []string{
		"run",
		"--no-profile",
		"--no-session",
		"-i", skillFile,
		"-q",
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.Command(GooseBin, args...)
	cmd.Dir = SourceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), hubEnv...)
	return cmd.Run()
}

func runOpenCode(model string, hubEnv []string, skillFile string) error {
	prompt, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("reading skill file: %w", err)
	}

	args := []string{
		"run",
		"--dangerously-skip-permissions",
		string(prompt),
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.Command("opencode", args...)
	cmd.Dir = SourceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), hubEnv...)
	return cmd.Run()
}

// PushBranch creates a new branch, commits all changes, and pushes.
func PushBranch(repoDir, branchName string) error {
	commands := [][]string{
		{"git", "checkout", "-b", branchName},
		{"git", "add", "-A"},
		{"git", "commit", "-m", fmt.Sprintf("konveyor: automated migration\n\nBranch: %s", branchName)},
		{"git", "push", "origin", branchName},
	}
	for _, c := range commands {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = repoDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running %s: %w", strings.Join(c, " "), err)
		}
	}
	return nil
}

// sanitize makes a string safe for use in a branch name.
func sanitize(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	return strings.ToLower(r.Replace(s))
}
