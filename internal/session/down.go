package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

func Down(stateDir string, tmuxClient tmux.Client, force bool) error {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return err
	}

	panes, err := tmuxClient.ListPanes(backgroundCtx(), cfg.Session.Name)
	if err != nil {
		return fmt.Errorf("list panes: %w", err)
	}

	if !force {
		for i := range panes {
			_ = tmuxClient.SendKeys(backgroundCtx(), panes[i].PaneID, "[tmuxicate] Session shutting down in 10s. Persist any needed reply with tmuxicate now.", true)
		}
		time.Sleep(10 * time.Second)
	}

	store := mailbox.NewStore(cfg.Session.StateDir)
	for _, agent := range cfg.Agents {
		activeDir := mailbox.InboxDir(cfg.Session.StateDir, agent.Name, protocol.FolderStateActive)
		entries, err := os.ReadDir(activeDir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read active inbox for %s: %w", agent.Name, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			receipt, err := store.ReadReceipt(agent.Name, protocol.MessageID(extractMessageID(entry.Name())))
			if err != nil {
				return err
			}
			lastErr := "session_stopped"
			if err := store.UpdateReceipt(agent.Name, receipt.MessageID, func(r *protocol.Receipt) {
				r.ClaimedBy = nil
				r.ClaimedAt = nil
				r.LastError = &lastErr
				r.Revision++
			}); err != nil {
				return err
			}
			if err := store.MoveReceipt(agent.Name, receipt.MessageID, protocol.FolderStateActive, protocol.FolderStateUnread); err != nil {
				return err
			}
		}
	}

	exists, err := tmuxClient.HasSession(backgroundCtx(), cfg.Session.Name)
	if err != nil {
		return fmt.Errorf("check session before kill: %w", err)
	}
	if exists {
		if err := tmuxClient.KillSession(backgroundCtx(), cfg.Session.Name); err != nil {
			return fmt.Errorf("kill session: %w", err)
		}
	}

	payload := map[string]any{
		"session":     cfg.Session.Name,
		"shutdown_at": time.Now().UTC().Format(time.RFC3339Nano),
		"force":       force,
		"state_dir":   cfg.Session.StateDir,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal shutdown file: %w", err)
	}

	path := filepath.Join(cfg.Session.StateDir, "runtime", "last_shutdown.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write shutdown file: %w", err)
	}

	return nil
}

func loadResolvedConfigFromStateDir(stateDir string) (*config.ResolvedConfig, error) {
	path := filepath.Join(stateDir, "config.resolved.yaml")
	return config.LoadResolved(path)
}

func extractMessageID(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if idx := strings.Index(base, "-"); idx >= 0 && idx+1 < len(base) {
		return base[idx+1:]
	}
	return base
}
