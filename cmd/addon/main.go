package main

import (
	"bytes"
	"fmt"
	"io"
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

	PalletBin = "/usr/bin/pallet"
	GooseBin  = "/usr/bin/goose"
)

// Data matches the TaskGroup data shape sent by the tackle2-ui migrate modal
// for kind "migration".
type Data struct {
	Agent  AgentConfig `json:"agent"`
	Plan   PlanConfig  `json:"plan"`
	Branch string      `json:"branch"`
}

type AgentConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Pallet      *PalletConfig     `json:"pallet,omitempty"`
	ModelConfig *AgentModelConfig `json:"modelConfig,omitempty"`
}

type AgentModelConfig struct {
	ProviderType string `json:"provider_type,omitempty"`
	URL          string `json:"url,omitempty"`
	Model        string `json:"model,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
}

type PlanConfig struct {
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
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

func main() {
	addon.Run(func() (err error) {
		d := &Data{}
		err = addon.DataWith(d)
		if err != nil {
			return
		}
		if strings.TrimSpace(d.Branch) == "" {
			return fmt.Errorf("data.branch is required")
		}

		// Provider/model: payload first, env vars as fallback.
		var provider, model string
		if d.Agent.ModelConfig != nil {
			provider = d.Agent.ModelConfig.ProviderType
			model = d.Agent.ModelConfig.Model
		}
		if provider == "" {
			provider = os.Getenv("KAI_PROVIDER")
		}
		if model == "" {
			model = os.Getenv("KAI_MODEL")
		}
		agentName := os.Getenv("KAI_AGENT")
		if agentName == "" {
			agentName = "goose"
		}

		addon.Activity("Fetching application.")
		application, err := addon.Task.Application()
		if err != nil {
			return
		}

		sshAgent := ssh.Agent{}
		err = sshAgent.Start()
		if err != nil {
			return
		}

		addon.Activity("Cloning repository.")
		var rp repository.SCM
		rp, err = FetchRepository(application)
		if err != nil {
			return
		}

		// Write pallet.yaml into the workspace if the agent carries pallet config.
		if d.Agent.Pallet != nil && d.Agent.Pallet.YAML != "" {
			palletPath := path.Join(SourceDir, "pallet.yaml")
			addon.Activity("Writing pallet.yaml to %s (%d bytes).",
				palletPath, len(d.Agent.Pallet.YAML))
			if err = os.WriteFile(palletPath, []byte(d.Agent.Pallet.YAML), 0644); err != nil {
				return fmt.Errorf("writing pallet.yaml: %w", err)
			}
		} else {
			addon.Activity("No pallet config provided in agent payload; skipping pallet.yaml write.")
		}

		addon.Activity("Syncing skills.")
		if syncErr := PalletSync(SourceDir); syncErr != nil {
			addon.Activity("Pallet sync skipped: %v", syncErr)
		}

		// Write the user's plan markdown into the workspace.
		planPath := path.Join(SourceDir, ".kai-plan.md")
		if strings.TrimSpace(d.Plan.Markdown) == "" {
			addon.Activity("WARNING: plan markdown is empty (plan name=%q). Goose will run with no instructions.",
				d.Plan.Name)
		}
		addon.Activity("Writing plan %q to %s (%d bytes).",
			d.Plan.Name, planPath, len(d.Plan.Markdown))
		if err = os.WriteFile(planPath, []byte(d.Plan.Markdown), 0644); err != nil {
			return fmt.Errorf("writing plan markdown: %w", err)
		}
		if info, statErr := os.Stat(planPath); statErr == nil {
			addon.Activity("Plan written: %s (%d bytes on disk).", planPath, info.Size())
		}

		// Environment for the agent subprocess so fetch-analysis can reach the Hub.
		hubEnv := []string{
			fmt.Sprintf("HUB_BASE_URL=%s", os.Getenv("HUB_BASE_URL")),
			fmt.Sprintf("HUB_TOKEN=%s", os.Getenv("TOKEN")),
			fmt.Sprintf("APP_ID=%d", application.ID),
		}

		addon.Activity("Running migration agent (%s) on plan %q.", agentName, d.Plan.Name)
		err = RunAgent(agentName, provider, model, hubEnv, planPath)
		if err != nil {
			return
		}

		addon.Activity("Switching to branch %s.", d.Branch)
		err = rp.Branch(d.Branch)
		if err != nil {
			return
		}
		addon.Activity("Committing and pushing migration to branch %s.", d.Branch)
		err = rp.Commit([]string{"."}, "konveyor: automated migration")
		if err != nil {
			return
		}

		addon.Activity("Migration complete. Branch: %s", d.Branch)
		return
	})
}

// run executes a command with full logging: the exact argv, the working dir,
// the exit code on failure, and a tail of combined stdout+stderr on failure.
// Stdout/stderr are still streamed to the addon's process output, so pod logs
// keep the full output, while the task's activity log gets enough to debug.
func run(workDir string, env []string, name string, args ...string) error {
	addon.Activity("$ %s %s  (cwd=%s)", name, strings.Join(args, " "), workDir)
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir
	var capture bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &capture)
	cmd.Stderr = io.MultiWriter(os.Stderr, &capture)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	err := cmd.Run()
	if err != nil {
		const maxTail = 4096
		out := capture.Bytes()
		if len(out) > maxTail {
			out = out[len(out)-maxTail:]
		}
		if len(out) > 0 {
			addon.Activity("output (last %d bytes):\n%s", len(out), string(out))
		}
		if ee, ok := err.(*exec.ExitError); ok {
			addon.Activity("command failed: %s %s — exit %d",
				name, strings.Join(args, " "), ee.ExitCode())
		} else {
			addon.Activity("command failed: %s %s — %v",
				name, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// PalletSync runs `pallet sync .` in the workspace directory.
func PalletSync(workDir string) error {
	if _, err := os.Stat(PalletBin); os.IsNotExist(err) {
		return fmt.Errorf("pallet binary not found at %s", PalletBin)
	}
	return run(workDir, nil, PalletBin, "sync", ".")
}

// RunAgent executes goose or opencode against the user's plan markdown.
func RunAgent(agentName, provider, model string, hubEnv []string, planFile string) error {
	switch strings.ToLower(agentName) {
	case "opencode":
		return runOpenCode(model, hubEnv, planFile)
	default:
		return runGoose(provider, model, hubEnv, planFile)
	}
}

func runGoose(provider, model string, hubEnv []string, planFile string) error {
	args := []string{
		"run",
		"--no-profile",
		"--no-session",
		"-i", planFile,
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	// GOOSE_MODE=auto auto-approves tool calls so goose runs non-interactively.
	env := append([]string{"GOOSE_MODE=auto"}, hubEnv...)
	return run(SourceDir, env, GooseBin, args...)
}

func runOpenCode(model string, hubEnv []string, planFile string) error {
	prompt, err := os.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("reading plan file: %w", err)
	}
	args := []string{
		"run",
		"--dangerously-skip-permissions",
		string(prompt),
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return run(SourceDir, hubEnv, "opencode", args...)
}

