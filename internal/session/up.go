package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
	"gopkg.in/yaml.v3"
)

var startBackgroundDaemonFn = startBackgroundDaemon

func Up(cfg *config.ResolvedConfig, tmuxClient tmux.Client) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if tmuxClient == nil {
		return fmt.Errorf("tmux client is required")
	}

	if err := createStateTree(cfg); err != nil {
		return err
	}
	if err := writeResolvedConfig(cfg); err != nil {
		return err
	}
	if err := generateAgentArtifacts(cfg); err != nil {
		return err
	}

	paneAgents, err := paneManagedAgents(cfg)
	if err != nil {
		return err
	}
	if len(paneAgents) == 0 {
		return fmt.Errorf("no local pane-backed agent configured for tmux session startup")
	}

	exists, err := tmuxClient.HasSession(backgroundCtx(), cfg.Session.Name)
	if err != nil {
		return fmt.Errorf("check existing session: %w", err)
	}
	if exists {
		return fmt.Errorf("tmux session %q already exists", cfg.Session.Name)
	}

	paneIDs, err := startPanes(cfg, tmuxClient, paneAgents)
	if err != nil {
		return err
	}

	if err := applyPaneMetadata(cfg, tmuxClient, paneAgents, paneIDs); err != nil {
		return err
	}
	if err := applyLayout(cfg, tmuxClient); err != nil {
		return err
	}
	if err := enableTranscripts(cfg, tmuxClient, paneAgents, paneIDs); err != nil {
		return err
	}
	if err := writeReadyFile(cfg, paneIDs); err != nil {
		return err
	}
	if err := startBackgroundDaemonFn(cfg); err != nil {
		return err
	}

	return nil
}

func createStateTree(cfg *config.ResolvedConfig) error {
	dirs := make([]string, 0, 9+len(cfg.Agents)*9)
	dirs = append(dirs,
		cfg.Session.StateDir,
		filepath.Join(cfg.Session.StateDir, "logs"),
		filepath.Join(cfg.Session.StateDir, "runtime"),
		filepath.Join(cfg.Session.StateDir, "state"),
		mailbox.MessagesDir(cfg.Session.StateDir),
		mailbox.StagingDir(cfg.Session.StateDir),
		mailbox.OrphanedMessagesDir(cfg.Session.StateDir),
		mailbox.LocksDir(cfg.Session.StateDir),
		filepath.Join(cfg.Session.StateDir, "locks", "receipts"),
	)

	for i := range cfg.Agents {
		name := cfg.Agents[i].Name
		dirs = append(dirs,
			mailbox.AgentDir(cfg.Session.StateDir, name),
			filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, name), "adapter"),
			filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, name), "events"),
			filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, name), "transcripts"),
			mailbox.InboxDir(cfg.Session.StateDir, name, "unread"),
			mailbox.InboxDir(cfg.Session.StateDir, name, "active"),
			mailbox.InboxDir(cfg.Session.StateDir, name, "done"),
			mailbox.InboxDir(cfg.Session.StateDir, name, "dead"),
			mailbox.ReceiptLocksDir(cfg.Session.StateDir, name),
		)
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create state dir %s: %w", dir, err)
		}
	}

	return nil
}

func writeResolvedConfig(cfg *config.ResolvedConfig) error {
	data, err := yaml.Marshal(cfg.Config)
	if err != nil {
		return fmt.Errorf("marshal resolved config: %w", err)
	}

	path := filepath.Join(cfg.Session.StateDir, "config.resolved.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write resolved config: %w", err)
	}

	return nil
}

func generateAgentArtifacts(cfg *config.ResolvedConfig) error {
	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		agentDir := mailbox.AgentDir(cfg.Session.StateDir, agent.Name)
		bootstrapPath := filepath.Join(agentDir, "adapter", "bootstrap.txt")
		runPath := filepath.Join(agentDir, "adapter", "run.sh")

		bootstrap := renderBootstrap(cfg, agent)
		if err := os.WriteFile(bootstrapPath, []byte(bootstrap), 0o644); err != nil {
			return fmt.Errorf("write bootstrap for %s: %w", agent.Name, err)
		}

		runScript := renderRunScript(cfg, agent, bootstrapPath)
		if err := os.WriteFile(runPath, []byte(runScript), 0o755); err != nil {
			return fmt.Errorf("write run.sh for %s: %w", agent.Name, err)
		}
		if err := os.Chmod(runPath, 0o755); err != nil {
			return fmt.Errorf("chmod run.sh for %s: %w", agent.Name, err)
		}
	}

	return nil
}

func startPanes(cfg *config.ResolvedConfig, tmuxClient tmux.Client, paneAgents []*config.AgentConfig) (map[string]string, error) {
	paneIDs := make(map[string]string, len(paneAgents))

	mainAgent := paneAgents[0]
	for _, agent := range paneAgents {
		if agent.Pane.Slot == "main" {
			mainAgent = agent
			break
		}
	}

	mainRunPath := filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, mainAgent.Name), "adapter", "run.sh")
	mainPaneID, err := tmuxClient.NewSession(backgroundCtx(), tmux.SessionSpec{
		Name:           cfg.Session.Name,
		WindowName:     cfg.Session.WindowName,
		StartDirectory: mainAgent.Workdir,
		Command:        fmt.Sprintf("bash -lc 'exec %q'", mainRunPath),
	})
	if err != nil {
		return nil, fmt.Errorf("create tmux session: %w", err)
	}
	paneIDs[mainAgent.Name] = mainPaneID

	if cfg.Session.Layout == "triad" {
		if err := startTriadPanes(cfg, tmuxClient, paneAgents, paneIDs, mainAgent.Name, mainPaneID); err != nil {
			return nil, err
		}
	} else {
		if err := startDefaultPanes(cfg, tmuxClient, paneAgents, paneIDs, mainAgent.Name, mainPaneID); err != nil {
			return nil, err
		}
	}

	return paneIDs, nil
}

func startTriadPanes(cfg *config.ResolvedConfig, tmuxClient tmux.Client, paneAgents []*config.AgentConfig, paneIDs map[string]string, mainAgentName, mainPaneID string) error {
	rightTopPaneID := ""
	for _, agent := range paneAgents {
		if agent.Name == mainAgentName {
			continue
		}
		runPath := filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, agent.Name), "adapter", "run.sh")
		spec := tmux.SplitSpec{
			TargetPane:     mainPaneID,
			StartDirectory: agent.Workdir,
			Command:        fmt.Sprintf("bash -lc 'exec %q'", runPath),
		}

		switch agent.Pane.Slot {
		case "right-top":
			spec.Direction = "h"
			spec.Percentage = 35
			paneID, err := tmuxClient.SplitPane(backgroundCtx(), spec)
			if err != nil {
				return fmt.Errorf("split right-top pane: %w", err)
			}
			paneIDs[agent.Name] = paneID
			rightTopPaneID = paneID
		case "right-bottom":
			if rightTopPaneID == "" {
				spec.Direction = "h"
				spec.Percentage = 35
			} else {
				spec.TargetPane = rightTopPaneID
				spec.Direction = "v"
				spec.Percentage = 50
			}
			paneID, err := tmuxClient.SplitPane(backgroundCtx(), spec)
			if err != nil {
				return fmt.Errorf("split right-bottom pane: %w", err)
			}
			paneIDs[agent.Name] = paneID
		default:
			spec.Direction = "v"
			spec.Percentage = 50
			paneID, err := tmuxClient.SplitPane(backgroundCtx(), spec)
			if err != nil {
				return fmt.Errorf("split extra pane for %s: %w", agent.Name, err)
			}
			paneIDs[agent.Name] = paneID
		}
	}

	return nil
}

func startDefaultPanes(cfg *config.ResolvedConfig, tmuxClient tmux.Client, paneAgents []*config.AgentConfig, paneIDs map[string]string, mainAgentName, mainPaneID string) error {
	for _, agent := range paneAgents {
		if agent.Name == mainAgentName {
			continue
		}
		runPath := filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, agent.Name), "adapter", "run.sh")
		paneID, err := tmuxClient.SplitPane(backgroundCtx(), tmux.SplitSpec{
			TargetPane:     mainPaneID,
			Direction:      "v",
			Percentage:     50,
			StartDirectory: agent.Workdir,
			Command:        fmt.Sprintf("bash -lc 'exec %q'", runPath),
		})
		if err != nil {
			return fmt.Errorf("split pane for %s: %w", agent.Name, err)
		}
		paneIDs[agent.Name] = paneID
	}

	return nil
}

func applyPaneMetadata(cfg *config.ResolvedConfig, tmuxClient tmux.Client, paneAgents []*config.AgentConfig, paneIDs map[string]string) error {
	windowTarget := fmt.Sprintf("%s:%s", cfg.Session.Name, cfg.Session.WindowName)
	if err := tmuxClient.SetSessionOption(backgroundCtx(), cfg.Session.Name, "@tmuxicate-state-dir", cfg.Session.StateDir); err != nil {
		return fmt.Errorf("set session option: %w", err)
	}
	if err := tmuxClient.SetSessionOption(backgroundCtx(), cfg.Session.Name, "@tmuxicate-window", windowTarget); err != nil {
		return fmt.Errorf("set session window option: %w", err)
	}

	for _, agent := range paneAgents {
		paneID := paneIDs[agent.Name]
		if err := tmuxClient.SetPaneTitle(backgroundCtx(), paneID, fmt.Sprintf("%s(%s)", agent.Name, agent.Alias)); err != nil {
			return fmt.Errorf("set pane title for %s: %w", agent.Name, err)
		}
		if err := tmuxClient.SetPaneOption(backgroundCtx(), paneID, "@tmuxicate-agent", agent.Name); err != nil {
			return fmt.Errorf("set pane agent option for %s: %w", agent.Name, err)
		}
		if err := tmuxClient.SetPaneOption(backgroundCtx(), paneID, "@tmuxicate-alias", agent.Alias); err != nil {
			return fmt.Errorf("set pane alias option for %s: %w", agent.Name, err)
		}
		if err := tmuxClient.SetPaneOption(backgroundCtx(), paneID, "@tmuxicate-adapter", agent.Adapter); err != nil {
			return fmt.Errorf("set pane adapter option for %s: %w", agent.Name, err)
		}
		if err := tmuxClient.SetPaneOption(backgroundCtx(), paneID, "@tmuxicate-pane-slot", agent.Pane.Slot); err != nil {
			return fmt.Errorf("set pane slot option for %s: %w", agent.Name, err)
		}
		if err := tmuxClient.SetPaneOption(backgroundCtx(), paneID, "@tmuxicate-session", cfg.Session.Name); err != nil {
			return fmt.Errorf("set pane session option for %s: %w", agent.Name, err)
		}
	}

	return nil
}

func applyLayout(cfg *config.ResolvedConfig, tmuxClient tmux.Client) error {
	window := fmt.Sprintf("%s:%s", cfg.Session.Name, cfg.Session.WindowName)
	layout := cfg.Session.Layout
	if layout == "triad" {
		layout = "main-vertical"
	}

	if err := tmuxClient.SelectLayout(backgroundCtx(), window, layout); err != nil {
		return fmt.Errorf("select layout: %w", err)
	}

	return nil
}

func enableTranscripts(cfg *config.ResolvedConfig, tmuxClient tmux.Client, paneAgents []*config.AgentConfig, paneIDs map[string]string) error {
	for _, agent := range paneAgents {
		transcriptPath := filepath.Join(mailbox.AgentDir(cfg.Session.StateDir, agent.Name), "transcripts", "raw.ansi.log")
		if err := os.WriteFile(transcriptPath, []byte{}, 0o644); err != nil {
			return fmt.Errorf("create transcript file for %s: %w", agent.Name, err)
		}
		pipeCmd := fmt.Sprintf("cat >> %q", transcriptPath)
		if err := tmuxClient.PipePane(backgroundCtx(), paneIDs[agent.Name], pipeCmd); err != nil {
			return fmt.Errorf("pipe pane for %s: %w", agent.Name, err)
		}
	}

	return nil
}

func paneManagedAgents(cfg *config.ResolvedConfig) ([]*config.AgentConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	managed := make([]*config.AgentConfig, 0, len(cfg.Agents))
	for i := range cfg.Agents {
		target, err := resolveExecutionTarget(cfg, &cfg.Agents[i])
		if err != nil {
			return nil, err
		}
		if target.Kind == "local" && target.PaneBacked {
			managed = append(managed, &cfg.Agents[i])
		}
	}

	return managed, nil
}

func writeReadyFile(cfg *config.ResolvedConfig, paneIDs map[string]string) error {
	agents := make(map[string]string, len(paneIDs))
	for k, v := range paneIDs {
		agents[k] = v
	}

	payload := map[string]any{
		"session":    cfg.Session.Name,
		"state":      "ready",
		"started_at": time.Now().UTC().Format(time.RFC3339Nano),
		"agents":     agents,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ready file: %w", err)
	}

	path := filepath.Join(cfg.Session.StateDir, "runtime", "ready.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write ready file: %w", err)
	}

	return nil
}

func renderBootstrap(cfg *config.ResolvedConfig, agent *config.AgentConfig) string {
	var teamLines []string
	for _, teammateName := range agent.Teammates {
		for i := range cfg.Agents {
			if cfg.Agents[i].Name == teammateName {
				teamLines = append(teamLines, fmt.Sprintf("- %s (alias: %s): %s", cfg.Agents[i].Name, cfg.Agents[i].Alias, cfg.Agents[i].Role.String()))
				break
			}
		}
	}

	text := fmt.Sprintf(`tmuxicate bootstrap

You are running inside a tmuxicate-managed tmux pane.

Identity
- Agent name: %s
- Alias: %s
- Session: %s
- Role: %s

Team
%s

Communication model
- The tmuxicate mailbox is the source of truth.
- Short lines injected into this pane are notifications only.
- Read a message with: tmuxicate read <message-id>
- List unread messages with: tmuxicate inbox --unread
- Reply with: tmuxicate reply <message-id> --stdin
- Accept a task with: tmuxicate task accept <message-id>
- Mark waiting with: tmuxicate task wait <message-id> --on <agent> --reason "<reason>"
- Mark blocked with: tmuxicate task block <message-id> --on human --reason "<reason>"
- Mark done with: tmuxicate task done <message-id> --summary "<one line>"

Working rules
- Stay within your role unless explicitly reassigned.
- Keep replies concise and specific.
- If instructions conflict, ask the coordinator instead of choosing silently.
- If you suspect pending work and have no notification, run: tmuxicate inbox --unread

Extra instructions
%s
`, agent.Name, agent.Alias, cfg.Session.Name, agent.Role.String(), strings.Join(teamLines, "\n"), strings.TrimSpace(agent.Bootstrap.ExtraInstructions))

	return text
}

func renderRunScript(cfg *config.ResolvedConfig, agent *config.AgentConfig, bootstrapPath string) string {
	envLines := make([]string, 0, len(cfg.Defaults.Env)+4)
	for k, v := range cfg.Defaults.Env {
		envLines = append(envLines, fmt.Sprintf("export %s=%q", k, v))
	}

	envLines = append(envLines,
		fmt.Sprintf("export TMUXICATE_SESSION=%q", cfg.Session.Name),
		fmt.Sprintf("export TMUXICATE_AGENT=%q", agent.Name),
		fmt.Sprintf("export TMUXICATE_ALIAS=%q", agent.Alias),
		fmt.Sprintf("export TMUXICATE_STATE_DIR=%q", cfg.Session.StateDir),
	)

	var execLine string
	switch agent.Adapter {
	case "codex":
		execLine = fmt.Sprintf("exec %s --no-alt-screen \"$(cat %q)\"", agent.Command, bootstrapPath)
	case "claude-code":
		execLine = fmt.Sprintf("exec %s --append-system-prompt \"$(cat %q)\" -n %q", agent.Command, bootstrapPath, fmt.Sprintf("%s@%s", agent.Name, cfg.Session.Name))
	default:
		execLine = fmt.Sprintf("exec %s", agent.Command)
	}

	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cd %q
%s
%s
`, agent.Workdir, strings.Join(envLines, "\n"), execLine)
}

func backgroundCtx() context.Context {
	return context.Background()
}

func startBackgroundDaemon(cfg *config.ResolvedConfig) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	stderrPath := filepath.Join(cfg.Session.StateDir, "logs", "serve.stderr.log")
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open serve stderr log: %w", err)
	}
	defer stderrFile.Close()

	cmd := exec.Command(exe, "serve", "--state-dir", cfg.Session.StateDir)
	cmd.Dir = cfg.Session.Workspace
	cmd.Env = os.Environ()
	cmd.Stdout = stderrFile
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background daemon: %w", err)
	}

	return cmd.Process.Release()
}
