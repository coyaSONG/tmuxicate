package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/adapter"
	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
	"github.com/fsnotify/fsnotify"
)

type Daemon struct {
	stateDir string
	tmux     tmux.Client
	cfg      *config.ResolvedConfig
	store    *mailbox.Store
	adapters map[string]adapter.Adapter
	watcher  *fsnotify.Watcher

	healthInterval    time.Duration
	heartbeatInterval time.Duration
	sweepInterval     time.Duration

	observed map[string]adapter.ReadyState
}

func NewDaemon(stateDir string, tmuxClient tmux.Client, cfg *config.ResolvedConfig) *Daemon {
	resolvedStateDir := stateDir
	if cfg != nil && cfg.Session.StateDir != "" {
		resolvedStateDir = cfg.Session.StateDir
	}

	d := &Daemon{
		stateDir:          resolvedStateDir,
		tmux:              tmuxClient,
		cfg:               cfg,
		store:             mailbox.NewStore(resolvedStateDir),
		adapters:          make(map[string]adapter.Adapter),
		healthInterval:    5 * time.Second,
		heartbeatInterval: 5 * time.Second,
		sweepInterval:     15 * time.Second,
		observed:          make(map[string]adapter.ReadyState),
	}

	if cfg != nil {
		d.adapters = d.buildAdapters()
	}

	return d
}

func (d *Daemon) Run(ctx context.Context) error {
	if d.cfg == nil {
		return fmt.Errorf("config is required")
	}
	if d.tmux == nil {
		return fmt.Errorf("tmux client is required")
	}

	if err := os.MkdirAll(filepath.Join(d.stateDir, "runtime"), 0o755); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(d.stateDir, "logs"), 0o755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}

	if err := d.writePID(); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(filepath.Join(d.stateDir, "runtime", "daemon.pid"))
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	d.watcher = watcher
	defer d.watcher.Close()

	for _, agentCfg := range d.cfg.Agents {
		dir := mailbox.InboxDir(d.stateDir, agentCfg.Name, protocol.FolderStateUnread)
		if err := d.watcher.Add(dir); err != nil {
			return fmt.Errorf("watch unread dir for %s: %w", agentCfg.Name, err)
		}
	}

	if err := d.fullSweep(ctx); err != nil {
		return err
	}
	if err := d.runHealthCheck(ctx); err != nil {
		return err
	}
	if err := d.writeHeartbeat(); err != nil {
		return err
	}

	healthTicker := time.NewTicker(d.healthInterval)
	defer healthTicker.Stop()
	heartbeatTicker := time.NewTicker(d.heartbeatInterval)
	defer heartbeatTicker.Stop()
	sweepTicker := time.NewTicker(d.sweepInterval)
	defer sweepTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-d.watcher.Events:
			if !ok {
				return nil
			}
			if err := d.handleFSEvent(ctx, ev); err != nil {
				d.logEvent("ERROR", "watch.handle", map[string]any{
					"path":  ev.Name,
					"op":    ev.Op.String(),
					"error": err.Error(),
				})
			}
		case err, ok := <-d.watcher.Errors:
			if !ok {
				return nil
			}
			d.logEvent("ERROR", "watch.error", map[string]any{"error": err.Error()})
		case <-healthTicker.C:
			if err := d.runHealthCheck(ctx); err != nil {
				d.logEvent("ERROR", "health.error", map[string]any{"error": err.Error()})
			}
		case <-heartbeatTicker.C:
			if err := d.writeHeartbeat(); err != nil {
				d.logEvent("ERROR", "heartbeat.error", map[string]any{"error": err.Error()})
			}
		case <-sweepTicker.C:
			if err := d.fullSweep(ctx); err != nil {
				d.logEvent("ERROR", "sweep.error", map[string]any{"error": err.Error()})
			}
		}
	}
}

func (d *Daemon) buildAdapters() map[string]adapter.Adapter {
	paneIDs := d.loadPaneIDs()
	adapters := make(map[string]adapter.Adapter, len(d.cfg.Agents))

	for _, agentCfg := range d.cfg.Agents {
		paneID := paneIDs[agentCfg.Name]
		if paneID == "" {
			continue
		}

		adapterCfg := adapter.GenericConfig{
			Command:       agentCfg.Command,
			QuietPeriod:   0,
			BootstrapMode: adapter.BootstrapModeNone,
		}

		switch agentCfg.Adapter {
		case "codex":
			adapterCfg.ReadyRegex = `(?m)^›(?:\s|$)`
			adapterCfg.QuietPeriod = 1500 * time.Millisecond
		case "claude-code":
			adapterCfg.ReadyRegex = `(?m)^❯\s*$`
			adapterCfg.QuietPeriod = 1200 * time.Millisecond
		}

		a, err := adapter.NewGenericAdapter(d.tmux, paneID, &adapterCfg)
		if err != nil {
			d.logEvent("ERROR", "adapter.create", map[string]any{
				"agent": agentCfg.Name,
				"error": err.Error(),
			})
			continue
		}
		adapters[agentCfg.Name] = a
	}

	return adapters
}

func (d *Daemon) handleFSEvent(ctx context.Context, ev fsnotify.Event) error {
	if ev.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Write) == 0 {
		return nil
	}
	if filepath.Ext(ev.Name) != ".yaml" {
		return nil
	}

	agent := filepath.Base(filepath.Dir(filepath.Dir(ev.Name)))
	msgID := protocol.MessageID(extractMessageID(filepath.Base(ev.Name)))
	if agent == "" || msgID == "" {
		return nil
	}

	return d.tryNotify(ctx, agent, msgID, false)
}

func (d *Daemon) fullSweep(ctx context.Context) error {
	for _, agentCfg := range d.cfg.Agents {
		dir := mailbox.InboxDir(d.stateDir, agentCfg.Name, protocol.FolderStateUnread)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read unread dir for %s: %w", agentCfg.Name, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}

			msgID := protocol.MessageID(extractMessageID(entry.Name()))
			if err := d.tryNotify(ctx, agentCfg.Name, msgID, true); err != nil {
				d.logEvent("ERROR", "notify.retry", map[string]any{
					"agent":      agentCfg.Name,
					"message_id": msgID,
					"error":      err.Error(),
				})
			}
		}
	}

	return nil
}

func (d *Daemon) tryNotify(ctx context.Context, agentName string, msgID protocol.MessageID, fromSweep bool) error {
	receipt, err := d.store.ReadReceipt(agentName, msgID)
	if err != nil {
		return nil
	}
	if receipt.FolderState != protocol.FolderStateUnread {
		return nil
	}

	now := time.Now().UTC()
	if fromSweep && receipt.NextRetryAt != nil && receipt.NextRetryAt.After(now) {
		return nil
	}

	env, _, err := d.store.ReadMessage(msgID)
	if err != nil {
		return err
	}

	agentAdapter := d.adapters[agentName]
	if agentAdapter == nil {
		return fmt.Errorf("no adapter configured for %s", agentName)
	}

	state, err := agentAdapter.Probe(ctx)
	if err != nil {
		return d.markNotifyFailure(agentName, msgID, fmt.Sprintf("probe failed: %v", err))
	}
	if err := d.writeObservedState(agentName, state); err != nil {
		d.logEvent("ERROR", "observed.write", map[string]any{
			"agent": agentName,
			"error": err.Error(),
		})
	}
	if state != adapter.ReadyStateReady {
		return d.markNotifyFailure(agentName, msgID, fmt.Sprintf("agent not ready: %s", state))
	}

	ref := adapter.MessageRef{
		ID:      env.ID,
		From:    string(env.From),
		Subject: env.Subject,
	}
	if err := agentAdapter.Notify(ctx, ref); err != nil {
		return d.markNotifyFailure(agentName, msgID, err.Error())
	}

	if err := d.store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
		at := time.Now().UTC()
		r.NotifyAttempts++
		r.LastNotifiedAt = &at
		r.NextRetryAt = nil
		r.LastError = nil
		r.Revision++
	}); err != nil {
		return err
	}

	d.logEvent("INFO", "notify.injected", map[string]any{
		"agent":      agentName,
		"message_id": msgID,
		"subject":    env.Subject,
	})
	return nil
}

func (d *Daemon) markNotifyFailure(agentName string, msgID protocol.MessageID, message string) error {
	nextRetry := time.Now().UTC().Add(d.cfg.Delivery.RetryInterval.Std())
	lastErr := message
	if err := d.store.UpdateReceipt(agentName, msgID, func(r *protocol.Receipt) {
		r.NextRetryAt = &nextRetry
		r.LastError = &lastErr
		r.Revision++
	}); err != nil {
		return err
	}

	d.logEvent("WARN", "notify.deferred", map[string]any{
		"agent":      agentName,
		"message_id": msgID,
		"error":      message,
		"retry_at":   nextRetry.Format(time.RFC3339Nano),
	})
	return nil
}

func (d *Daemon) runHealthCheck(ctx context.Context) error {
	for agentName, agentAdapter := range d.adapters {
		state, err := agentAdapter.Probe(ctx)
		if err != nil {
			state = adapter.ReadyStateUnknown
		}
		if err := d.writeObservedState(agentName, state); err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) writeObservedState(agentName string, state adapter.ReadyState) error {
	if d.observed[agentName] == state {
		return nil
	}
	d.observed[agentName] = state

	eventsDir := filepath.Join(mailbox.AgentDir(d.stateDir, agentName), "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return fmt.Errorf("create events dir: %w", err)
	}

	payload := map[string]any{
		"schema":         "tmuxicate/observed-state/v1",
		"ts":             time.Now().UTC().Format(time.RFC3339Nano),
		"agent":          agentName,
		"observed_state": string(state),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal observed state: %w", err)
	}

	path := filepath.Join(eventsDir, "observed.current.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write observed state: %w", err)
	}

	return nil
}

func (d *Daemon) writeHeartbeat() error {
	payload := map[string]any{
		"schema":     "tmuxicate/daemon-heartbeat/v1",
		"pid":        os.Getpid(),
		"session":    d.cfg.Session.Name,
		"state_dir":  d.stateDir,
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		"agents":     len(d.cfg.Agents),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	path := filepath.Join(d.stateDir, "runtime", "daemon.heartbeat.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}

	return nil
}

func (d *Daemon) writePID() error {
	path := filepath.Join(d.stateDir, "runtime", "daemon.pid")
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func (d *Daemon) loadPaneIDs() map[string]string {
	type readyFile struct {
		Agents map[string]string `json:"agents"`
	}

	path := filepath.Join(d.stateDir, "runtime", "ready.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}

	var payload readyFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return map[string]string{}
	}

	if payload.Agents == nil {
		return map[string]string{}
	}
	return payload.Agents
}

func (d *Daemon) logEvent(level, event string, fields map[string]any) {
	path := filepath.Join(d.stateDir, "logs", "serve.jsonl")
	record := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"event": event,
	}
	for k, v := range fields {
		record[k] = v
	}

	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func extractMessageID(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if idx := strings.Index(base, "-"); idx >= 0 && idx+1 < len(base) {
		return base[idx+1:]
	}
	return base
}
