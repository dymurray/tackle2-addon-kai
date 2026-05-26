package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/konveyor/tackle2-addon/repository"
	"github.com/konveyor/tackle2-addon/ssh"
	hub "github.com/konveyor/tackle2-hub/addon"
)

var (
	// Global addon instance
	addon = hub.Addon
	
	// Directory paths
	Dir       = ""
	SourceDir = ""

	PalletBin     = "/usr/bin/pallet"
	GooseBin      = "/usr/bin/goose"
	GooseHintsSrc = "/usr/share/goosehints"
)

// preambleIntro is prepended to every plan before it is handed to the AI
// agent. It tells the model there is no human in the loop — without this the
// model will sometimes end its response with a clarifying question, which
// causes `goose run -i` to exit without completing the migration.
const preambleIntro = `# Execution context (not part of the user plan)

You are running in a non-interactive automated environment. There is no human available to answer questions.

- Do NOT ask clarifying questions. Make reasonable assumptions and proceed.
- Do NOT request confirmation before taking actions. Execute directly.
- Do NOT propose multiple options for the user to pick from. Pick the best one and continue.
- Complete every step of the plan autonomously, then end with a brief summary of what you did.

`

// userPlanHeader separates the execution-context preamble (and any synced
// asset manifest) from the user plan body.
const userPlanHeader = `---

# User plan

`

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

		// Write the user's plan markdown into the workspace, prefixed with a
		// non-interactive preamble. `goose run -i <file>` exits as soon as the
		// model's response ends, so a plan that elicits a clarifying question
		// from the model causes the whole task to exit without doing the work.
		// The preamble tells the model up front that no human is available.
		planPath := path.Join(SourceDir, ".kai-plan.md")
		if strings.TrimSpace(d.Plan.Markdown) == "" {
			addon.Activity("WARNING: plan markdown is empty (plan name=%q). Goose will run with no instructions.",
				d.Plan.Name)
		}
		manifest, skillsN, memoriesN := buildSyncedAssetsSection(SourceDir)
		addon.Activity("Synced asset manifest: %d skill(s), %d memorie(s) under %s/.goose.",
			skillsN, memoriesN, SourceDir)
		planBody := preambleIntro + manifest + userPlanHeader + d.Plan.Markdown
		addon.Activity("Writing plan %q to %s (%d bytes user content + %d bytes preamble + %d bytes manifest).",
			d.Plan.Name, planPath, len(d.Plan.Markdown),
			len(preambleIntro)+len(userPlanHeader), len(manifest))
		if err = os.WriteFile(planPath, []byte(planBody), 0644); err != nil {
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

		if hints, readErr := os.ReadFile(GooseHintsSrc); readErr == nil {
			os.WriteFile(path.Join(SourceDir, ".goosehints"), hints, 0644)
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

var MempalaceURL = os.Getenv("MEMPALACE_URL")

func runGoose(provider, model string, hubEnv []string, planFile string) error {
	args := []string{
		"run",
		"--no-session",
		"--no-profile",
		"--with-builtin", "developer",
		"-i", planFile,
	}
	if MempalaceURL != "" {
		args = append(args, "--with-streamable-http-extension", MempalaceURL)
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	env := append([]string{
		// Auto-approve tool calls (no human in the loop).
		"GOOSE_MODE=auto",
		// Plain output for the pod log: no ANSI colors, no in-place TUI.
		"NO_COLOR=1",
		"TERM=dumb",
	}, hubEnv...)
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

// buildSyncedAssetsSection enumerates the skills and memories that pallet
// synced into workDir/.goose/ and returns a markdown manifest naming each one
// for the agent. Goose has no conditional skill loader, so without this
// manifest the synced files are invisible to the model. Returns "" with zero
// counts when nothing was synced.
func buildSyncedAssetsSection(workDir string) (manifest string, skillsCount, memoriesCount int) {
	skillsDir := filepath.Join(workDir, ".goose", "skills")
	memoriesDir := filepath.Join(workDir, ".goose", "memories")

	skills := readSyncedSkills(skillsDir)
	memories := readSyncedMemories(memoriesDir)
	skillsCount = len(skills)
	memoriesCount = len(memories)
	if skillsCount == 0 && memoriesCount == 0 {
		return "", 0, 0
	}

	var b strings.Builder
	b.WriteString("# Available skills and memories (synced by pallet)\n\n")
	b.WriteString("The files listed below were synced into this workspace before you started.\n")
	b.WriteString("They contain authoritative guidance for this task. Read any whose name or\n")
	b.WriteString("description matches the work in the user plan before acting; do NOT skip a\n")
	b.WriteString("skill whose description matches the task at hand.\n\n")

	if len(skills) > 0 {
		b.WriteString("## Skills (read the SKILL.md if relevant)\n\n")
		for _, s := range skills {
			fmt.Fprintf(&b, "- **%s** — %s (`%s`)\n", s.name, s.description, s.relPath)
		}
		b.WriteString("\n")
	}
	if len(memories) > 0 {
		b.WriteString("## Memories / rules (read in full before acting)\n\n")
		for _, m := range memories {
			fmt.Fprintf(&b, "- `%s`\n", m)
		}
		b.WriteString("\n")
	}
	return b.String(), skillsCount, memoriesCount
}

type syncedSkill struct {
	name        string
	description string
	relPath     string
}

func readSyncedSkills(skillsDir string) []syncedSkill {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	var out []syncedSkill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		name, desc := parseSkillFrontmatter(skillFile)
		if name == "" {
			name = e.Name()
		}
		if desc == "" {
			desc = "(no description)"
		}
		rel := filepath.Join(".goose", "skills", e.Name(), "SKILL.md")
		out = append(out, syncedSkill{name: name, description: desc, relPath: rel})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func readSyncedMemories(memoriesDir string) []string {
	entries, err := os.ReadDir(memoriesDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, filepath.Join(".goose", "memories", e.Name()))
	}
	sort.Strings(out)
	return out
}

// parseSkillFrontmatter reads the leading `---` YAML block of a SKILL.md and
// extracts the `name` and `description` fields. Pallet writes Agent-Skills-
// formatted SKILL.md files with frontmatter intact, so a tiny line scanner is
// enough — no YAML dependency. Description values that span multiple lines
// (folded `>` style) are joined with single spaces.
func parseSkillFrontmatter(path string) (name, description string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", ""
	}
	var (
		curKey  string
		descBuf []string
	)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		if isFrontmatterKey(line) {
			key, val := splitFrontmatterKey(line)
			curKey = key
			switch key {
			case "name":
				name = strings.Trim(val, "\"' ")
			case "description":
				v := strings.TrimSpace(strings.TrimPrefix(val, ">"))
				v = strings.Trim(v, "\"'")
				if v != "" {
					descBuf = append(descBuf, v)
				}
			}
			continue
		}
		if curKey == "description" && trimmed != "" {
			descBuf = append(descBuf, trimmed)
		}
	}
	description = strings.Join(descBuf, " ")
	return name, description
}

// isFrontmatterKey reports whether a line begins a top-level YAML mapping
// entry (e.g. "name: foo"). Indented continuation lines do not count.
func isFrontmatterKey(line string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
		return false
	}
	idx := strings.Index(line, ":")
	return idx > 0
}

func splitFrontmatterKey(line string) (key, val string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
}
