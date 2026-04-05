package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coyaSONG/tmuxicate/internal/tmux"
)

type PickOpts struct {
	Session string
	Emit    string // alias, name, pane-id
	Insert  string // send-target, raw
}

type ListPanesOpts struct {
	Session string
}

type PreviewPaneOpts struct {
	Session string
	PaneID  string
	Alias   string
}

func Pick(stateDir string, client tmux.Client, opts PickOpts) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf is required for pick but was not found in PATH: install it with 'brew install fzf' or see https://github.com/junegunn/fzf#installation")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	emit := opts.Emit
	if emit == "" {
		emit = "alias"
	}
	insert := opts.Insert
	if insert == "" {
		insert = "raw"
	}

	targetPane := os.Getenv("TMUXICATE_PICK_TARGET")
	if targetPane == "" {
		targetPane = os.Getenv("TMUX_PANE")
	}

	// Build the fzf pipeline that runs inside the popup.
	// fzf outputs the selected tab-delimited row; we extract the desired field.
	fzfCmd := fmt.Sprintf(
		`%s __list-panes --state-dir %q | `+
			`fzf --ansi `+
			`--delimiter=$'\t' `+
			`--with-nth=2,3,4,5,6,7 `+
			`--nth=2,3,7 `+
			`--prompt='agent> ' `+
			`--height=100%% `+
			`--layout=reverse `+
			`--border=rounded `+
			`--info=inline-right `+
			`--no-sort `+
			`--bind 'ctrl-r:reload(%s __list-panes --state-dir %q)' `+
			`--preview '%s __preview-pane --state-dir %q --pane {1} --alias {2}' `+
			`--preview-window 'right,65%%,wrap,border-left'`,
		exe, stateDir,
		exe, stateDir,
		exe, stateDir,
	)

	// Wrap fzf output handling: extract the desired column and insert into target pane.
	var shellCmd string
	switch insert {
	case "send-target":
		// Just print the selected value; caller handles insertion.
		shellCmd = fmt.Sprintf(
			`sel=$(%s) && [ -n "$sel" ] && echo "$sel" | cut -f%s`,
			fzfCmd, emitCutField(emit),
		)
	default: // "raw"
		if targetPane == "" {
			// No target pane; just print.
			shellCmd = fmt.Sprintf(
				`sel=$(%s) && [ -n "$sel" ] && echo "$sel" | cut -f%s`,
				fzfCmd, emitCutField(emit),
			)
		} else {
			// Extract value and paste into target pane via tmux buffer.
			shellCmd = fmt.Sprintf(
				`sel=$(%s) && [ -n "$sel" ] && val=$(echo "$sel" | cut -f%s) && `+
					`tmux set-buffer -- "@${val}" && `+
					`tmux paste-buffer -p -t %q`,
				fzfCmd, emitCutField(emit), targetPane,
			)
		}
	}

	ctx := context.Background()
	return client.DisplayPopup(ctx, &tmux.PopupSpec{
		TargetPane: targetPane,
		Title:      " pick agent ",
		Width:      "80%",
		Height:     "60%",
		Command:    shellCmd,
	})
}

func ListPanes(stateDir string, client tmux.Client, opts ListPanesOpts) (string, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	sessionName := opts.Session
	if sessionName == "" {
		sessionName = cfg.Session.Name
	}

	_, paneIDs, _ := readReadyFile(filepath.Join(stateDir, "runtime", "ready.json"))

	livePaneIDs := map[string]string{}
	hasSession, _ := client.HasSession(ctx, sessionName)
	if hasSession {
		panes, err := client.ListPanes(ctx, sessionName)
		if err == nil {
			for i := range panes {
				agentName, err := client.ShowPaneOption(ctx, panes[i].PaneID, "@tmuxicate-agent")
				if err == nil && strings.TrimSpace(agentName) != "" {
					livePaneIDs[agentName] = panes[i].PaneID
				}
			}
		}
	}

	var lines []string
	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		paneID := coalescePaneID(livePaneIDs[agent.Name], paneIDs[agent.Name])

		observed, _, _ := readObservedState(filepath.Join(stateDir, "agents", agent.Name, "events", "observed.current.json"))
		declared, _, _ := readDeclaredState(filepath.Join(stateDir, "agents", agent.Name, "events", "state.current.json"))

		unreadCount, _ := countReceiptFiles(filepath.Join(stateDir, "agents", agent.Name, "inbox", "unread"))

		role := agent.Role.String()
		if role == "" {
			role = "-"
		}

		line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d\t%s",
			paneID,
			agent.Alias,
			agent.Name,
			observed,
			declared,
			unreadCount,
			role,
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func PreviewPane(stateDir string, client tmux.Client, opts PreviewPaneOpts) (string, error) {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return "", err
	}

	// Find the agent by alias or pane ID.
	var agentName, alias, role string
	for i := range cfg.Agents {
		if cfg.Agents[i].Alias == opts.Alias || cfg.Agents[i].Name == opts.Alias {
			agentName = cfg.Agents[i].Name
			alias = cfg.Agents[i].Alias
			role = cfg.Agents[i].Role.String()
			break
		}
	}
	if agentName == "" {
		return fmt.Sprintf("unknown agent %q", opts.Alias), nil
	}

	paneID := opts.PaneID
	if paneID == "" || paneID == "-" {
		_, paneIDs, _ := readReadyFile(filepath.Join(stateDir, "runtime", "ready.json"))
		paneID = paneIDs[agentName]
	}

	// Read pane title from tmux if we have a live pane.
	paneTitle := "-"
	if paneID != "" && paneID != "-" {
		ctx := context.Background()
		sessionName := opts.Session
		if sessionName == "" {
			sessionName = cfg.Session.Name
		}
		panes, err := client.ListPanes(ctx, sessionName)
		if err == nil {
			for i := range panes {
				if panes[i].PaneID == paneID {
					paneTitle = panes[i].PaneTitle
					break
				}
			}
		}
	}

	observed, _, _ := readObservedState(filepath.Join(stateDir, "agents", agentName, "events", "observed.current.json"))
	declared, _, _ := readDeclaredState(filepath.Join(stateDir, "agents", agentName, "events", "state.current.json"))
	unreadCount, _ := countReceiptFiles(filepath.Join(stateDir, "agents", agentName, "inbox", "unread"))
	activeCount, _ := countReceiptFiles(filepath.Join(stateDir, "agents", agentName, "inbox", "active"))

	var b strings.Builder
	fmt.Fprintf(&b, "Alias:     %s\n", alias)
	fmt.Fprintf(&b, "Agent:     %s\n", agentName)
	fmt.Fprintf(&b, "Pane:      %s\n", coalescePaneID(paneID))
	fmt.Fprintf(&b, "Title:     %s\n", paneTitle)
	fmt.Fprintf(&b, "Role:      %s\n", role)
	fmt.Fprintf(&b, "Declared:  %s\n", declared)
	fmt.Fprintf(&b, "Observed:  %s\n", observed)
	fmt.Fprintf(&b, "Unread:    %d\n", unreadCount)
	fmt.Fprintf(&b, "Active:    %d\n", activeCount)
	fmt.Fprintf(&b, "\n--- last 20 lines ---\n")

	// Read last 20 lines from transcript.
	transcript := readLastTranscriptLines(stateDir, agentName, 20)
	if transcript == "" {
		b.WriteString("(no transcript)\n")
	} else {
		b.WriteString(transcript)
		if !strings.HasSuffix(transcript, "\n") {
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}

func readLastTranscriptLines(stateDir, agent string, n int) string {
	path := transcriptPath(stateDir, agent, false)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := stripANSI(string(data))
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// emitCutField maps emit mode to the tab-delimited column number.
func emitCutField(emit string) string {
	switch emit {
	case "name":
		return "3"
	case "pane-id":
		return "1"
	default: // "alias"
		return "2"
	}
}
