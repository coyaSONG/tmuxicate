package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"gopkg.in/yaml.v3"
)

type InitOpts struct {
	Dir      string
	Template string
	Force    bool
}

type detectedCLI struct {
	Name    string
	Command string
	Adapter string
}

func Init(opts InitOpts) error {
	targetDir := strings.TrimSpace(opts.Dir)
	if targetDir == "" {
		targetDir = "."
	}

	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolve init dir: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("create init dir: %w", err)
	}

	template := strings.TrimSpace(opts.Template)
	if template == "" {
		template = "triad"
	}
	if template != "minimal" && template != "triad" {
		return fmt.Errorf("unsupported template %q", template)
	}

	configPath := filepath.Join(absDir, "tmuxicate.yaml")
	if !opts.Force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("tmuxicate.yaml already exists at %s; use --force to overwrite", configPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat existing config: %w", err)
		}
	}

	clis := detectCLIs()
	cfg := buildInitConfig(absDir, template, clis)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal generated config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("write tmuxicate config: %w", err)
	}

	if err := ensureGitignoreEntry(absDir, ".tmuxicate/"); err != nil {
		return err
	}

	fmt.Println("Created tmuxicate.yaml")
	fmt.Println("Added .tmuxicate/ to .gitignore")
	fmt.Println("Next:")
	fmt.Println("  tmuxicate up")
	fmt.Println("  tmuxicate send pm \"Describe the first task\"")
	fmt.Println("  tmuxicate status")

	return nil
}

func detectCLIs() []detectedCLI {
	candidates := []detectedCLI{
		{Name: "claude", Command: "claude", Adapter: "claude-code"},
		{Name: "codex", Command: "codex", Adapter: "codex"},
		{Name: "gemini", Command: "gemini", Adapter: "generic"},
		{Name: "aider", Command: "aider", Adapter: "generic"},
	}

	var found []detectedCLI
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.Command); err == nil {
			found = append(found, candidate)
		}
	}
	if len(found) == 0 {
		found = append(found, detectedCLI{
			Name:    "bash",
			Command: "bash",
			Adapter: "generic",
		})
	}
	return found
}

func buildInitConfig(absDir, template string, clis []detectedCLI) config.Config {
	base := filepath.Base(absDir)
	if strings.TrimSpace(base) == "" || base == string(filepath.Separator) {
		base = "dev"
	}
	base = strings.ToLower(strings.ReplaceAll(base, " ", "-"))

	sessionName := "tmuxicate-" + base
	stateSuffix := base
	if template == "minimal" {
		stateSuffix = base + "-minimal"
	}

	cfg := config.Config{
		Version: 1,
		Session: config.SessionConfig{
			Name:       sessionName,
			Workspace:  ".",
			StateDir:   filepath.ToSlash(filepath.Join(".tmuxicate", "sessions", stateSuffix)),
			WindowName: "agents",
			Layout:     map[bool]string{true: "triad", false: "main-vertical"}[template == "triad"],
			Attach:     boolPtr(false),
		},
		Delivery: config.DeliveryConfig{
			Mode:          "notify_then_read",
			AckTimeout:    config.Duration(2 * 60 * 1e9),
			RetryInterval: config.Duration(30 * 1e9),
			MaxRetries:    3,
			AutoNotify:    boolPtr(true),
		},
		Transcript: config.TranscriptConfig{
			Mode: "pipe-pane",
			Dir:  filepath.ToSlash(filepath.Join(".tmuxicate", "sessions", stateSuffix, "transcripts")),
		},
		Routing: config.RoutingConfig{
			Coordinator: "coordinator",
		},
		Defaults: config.DefaultsConfig{
			Workdir: ".",
			Env: map[string]string{
				"TMUXICATE_SESSION": sessionName,
			},
			Notify: config.NotifyConfig{
				Enabled: boolPtr(true),
			},
		},
	}

	if template == "minimal" {
		cli := chooseCLI(clis, 0)
		cfg.Agents = []config.AgentConfig{
			agentTemplate("coordinator", "pm", cli, "Project coordinator", "main", []string{"worker"}, "Break work down, route tasks, and escalate ambiguity."),
			agentTemplate("worker", "dev", cli, "General implementation agent", "right-top", []string{"coordinator"}, "Focus on implementation and reply with concrete progress."),
		}
		return cfg
	}

	coordCLI := chooseCLI(clis, 0)
	backendCLI := chooseCLI(clis, 1)
	reviewerCLI := chooseCLI(clis, 2)
	cfg.Agents = []config.AgentConfig{
		agentTemplate("coordinator", "pm", coordCLI, "Project coordinator", "main", []string{"backend", "reviewer"}, "Break work down, route tasks, and resolve conflicts."),
		agentTemplate("backend", "api", backendCLI, "Backend implementer", "right-top", []string{"coordinator", "reviewer"}, "Focus on code changes and targeted verification."),
		agentTemplate("reviewer", "review", reviewerCLI, "Reviewer", "right-bottom", []string{"coordinator", "backend"}, "Review plans and changes for bugs, regressions, and missing tests."),
	}
	return cfg
}

func chooseCLI(clis []detectedCLI, index int) detectedCLI {
	if len(clis) == 0 {
		return detectedCLI{Name: "bash", Command: "bash", Adapter: "generic"}
	}
	return clis[index%len(clis)]
}

func agentTemplate(name, alias string, cli detectedCLI, role, slot string, teammates []string, extra string) config.AgentConfig {
	return config.AgentConfig{
		Name:    name,
		Alias:   alias,
		Adapter: cli.Adapter,
		Command: cli.Command,
		Role:    role,
		Pane: config.PaneConfig{
			Slot: slot,
		},
		Teammates: teammates,
		Bootstrap: config.BootstrapConfig{
			ExtraInstructions: extra,
		},
	}
}

func ensureGitignoreEntry(dir, entry string) error {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	content := string(data)
	if strings.Contains(content, entry) {
		return nil
	}

	var builder strings.Builder
	if strings.TrimSpace(content) != "" {
		builder.WriteString(strings.TrimRight(content, "\n"))
		builder.WriteString("\n\n")
	}
	builder.WriteString("# tmuxicate runtime state\n")
	builder.WriteString(entry)
	builder.WriteString("\n")

	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	return nil
}

func boolPtr(v bool) *bool {
	return &v
}
