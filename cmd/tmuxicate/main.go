package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
	tmuxruntime "github.com/coyaSONG/tmuxicate/internal/runtime"
	"github.com/coyaSONG/tmuxicate/internal/session"
	"github.com/coyaSONG/tmuxicate/internal/tmux"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "tmuxicate",
		Short: "Multi-agent collaboration in tmux",
	}

	rootCmd.AddCommand(
		newUpCmd(),
		newDownCmd(),
		newSendCmd(),
		newInboxCmd(),
		newReadCmd(),
		newReplyCmd(),
		newNextCmd(),
		newTaskCmd(),
		newStatusCmd(),
		newLogCmd(),
		newInitCmd(),
		newPickCmd(),
		newListPanesCmd(),
		newPreviewPaneCmd(),
		newServeCmd(),
	)

	return rootCmd
}

func newUpCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start a tmuxicate session",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			client := tmux.NewRealClient("")
			if err := session.Up(cfg, client); err != nil {
				return err
			}

			fmt.Printf("session %s started\n", cfg.Session.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	return cmd
}

func newDownCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop a tmuxicate session",
		RunE: func(_ *cobra.Command, _ []string) error {
			if stateDir == "" {
				cfg, err := config.LoadResolved(configPath)
				if err != nil {
					return err
				}
				stateDir = cfg.Session.StateDir
			}

			client := tmux.NewRealClient("")
			if err := session.Down(stateDir, client, force); err != nil {
				return err
			}

			fmt.Printf("session at %s stopped\n", stateDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().BoolVar(&force, "force", false, "force shutdown without grace period")
	return cmd
}

func newSendCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var subject string
	var kind string

	cmd := &cobra.Command{
		Use:   "send <agent> <message>",
		Short: "Send a message to an agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if stateDir == "" {
				cfg, err := config.LoadResolved(configPath)
				if err != nil {
					return err
				}
				stateDir = cfg.Session.StateDir
			}

			store := mailbox.NewStore(stateDir)
			body := strings.Join(args[1:], " ")
			msgID, err := session.Send(stateDir, store, args[0], body, session.SendOpts{
				Subject: subject,
				Kind:    protocol.Kind(kind),
			})
			if err != nil {
				return err
			}

			fmt.Println(msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&subject, "subject", "", "optional message subject")
	cmd.Flags().StringVar(&kind, "kind", string(protocol.KindTask), "message kind")
	return cmd
}

func newTaskCmd() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage task state",
		Run:   stubRun,
	}

	taskCmd.AddCommand(
		newTaskAcceptCmd(),
		newTaskWaitCmd(),
		newTaskBlockCmd(),
		newTaskDoneCmd(),
	)

	return taskCmd
}

func newInboxCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var unreadOnly bool
	var all bool

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List inbox messages",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}
			if all {
				unreadOnly = false
			}

			entries, err := session.Inbox(resolvedStateDir, resolvedAgent, unreadOnly)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("inbox empty")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SEQ\tPRI\tSTATE\tKIND\tFROM\tTHREAD\tAGE\tSUBJECT")
			for i := range entries {
				subject := entries[i].Subject
				if subject == "" {
					subject = "-"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					entries[i].Seq,
					entries[i].Priority,
					entries[i].State,
					entries[i].Kind,
					entries[i].From,
					entries[i].Thread,
					formatAge(entries[i].Age),
					subject,
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().BoolVar(&unreadOnly, "unread", true, "show unread and active messages only")
	cmd.Flags().BoolVar(&all, "all", false, "show unread, active, and done messages")
	return cmd
}

func newReadCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string

	cmd := &cobra.Command{
		Use:   "read <message-id>",
		Short: "Read a message",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			result, err := session.ReadMsg(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]))
			if err != nil {
				return err
			}

			printReadResult(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	return cmd
}

func newReplyCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var bodyFile string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "reply <message-id>",
		Short: "Reply to a message",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			body, err := readReplyBody(bodyFile, useStdin)
			if err != nil {
				return err
			}

			store := mailbox.NewStore(resolvedStateDir)
			msgID, err := session.Reply(resolvedStateDir, store, resolvedAgent, protocol.MessageID(args[0]), body)
			if err != nil {
				return err
			}

			fmt.Println(msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to reply body file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read reply body from stdin")
	return cmd
}

func newNextCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Read the next unread message",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			result, err := session.Next(resolvedStateDir, resolvedAgent)
			if err != nil {
				return err
			}

			printReadResult(result)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	return cmd
}

func newTaskAcceptCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string

	cmd := &cobra.Command{
		Use:   "accept <message-id>",
		Short: "Accept a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			if err := session.TaskAccept(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0])); err != nil {
				return err
			}
			fmt.Println("accepted")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	return cmd
}

func newTaskWaitCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var on string
	var reason string

	cmd := &cobra.Command{
		Use:   "wait <message-id>",
		Short: "Mark a task as waiting",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			if err := session.TaskWait(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]), on, reason); err != nil {
				return err
			}
			fmt.Println("waiting")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&on, "on", "", "target agent or dependency being waited on")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for waiting")
	return cmd
}

func newTaskBlockCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var on string
	var reason string

	cmd := &cobra.Command{
		Use:   "block <message-id>",
		Short: "Mark a task as blocked",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			if err := session.TaskBlock(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]), on, reason); err != nil {
				return err
			}
			fmt.Println("blocked")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&on, "on", "human", "target agent or dependency causing the block")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for the block")
	return cmd
}

func newTaskDoneCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var summary string

	cmd := &cobra.Command{
		Use:   "done <message-id>",
		Short: "Mark a task as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}
			resolvedAgent, err := resolveAgent(agent)
			if err != nil {
				return err
			}

			if err := session.TaskDone(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]), summary); err != nil {
				return err
			}
			fmt.Println("done")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&summary, "summary", "", "optional completion summary")
	return cmd
}

func newServeCmd() *cobra.Command {
	var configPath string
	var stateDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the tmuxicate daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			cfg, err := config.LoadResolved(filepathJoin(resolvedStateDir, "config.resolved.yaml"))
			if err != nil {
				return err
			}

			daemon := tmuxruntime.NewDaemon(resolvedStateDir, tmux.NewRealClient(""), cfg)
			return daemon.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	return cmd
}

func newInitCmd() *cobra.Command {
	var dir string
	var template string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize tmuxicate configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			return session.Init(session.InitOpts{
				Dir:      dir,
				Template: template,
				Force:    force,
			})
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "directory to initialize")
	cmd.Flags().StringVar(&template, "template", "triad", "template to generate: minimal or triad")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing tmuxicate.yaml")
	return cmd
}

func newStatusCmd() *cobra.Command {
	var configPath string
	var stateDir string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show session status",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			report, err := session.Status(resolvedStateDir, tmux.NewRealClient(""))
			if err != nil {
				return err
			}

			printStatusReport(report)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	return cmd
}

func newLogCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var tail int
	var follow bool
	var all bool
	var raw bool
	var eventsOnly bool

	cmd := &cobra.Command{
		Use:   "log [agent]",
		Short: "Show transcripts and events",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			agent := ""
			if len(args) > 0 {
				agent = args[0]
			}

			return session.LogView(resolvedStateDir, agent, session.LogOpts{
				Tail:       tail,
				Follow:     follow,
				All:        all,
				Raw:        raw,
				EventsOnly: eventsOnly,
			})
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().IntVar(&tail, "tail", 100, "number of lines to show")
	cmd.Flags().BoolVar(&follow, "follow", false, "follow log output")
	cmd.Flags().BoolVar(&all, "all", false, "show logs for all agents")
	cmd.Flags().BoolVar(&raw, "raw", false, "show raw transcript output")
	cmd.Flags().BoolVar(&eventsOnly, "events", false, "show structured events only")
	return cmd
}

func newPickCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var emit string
	var insert string

	cmd := &cobra.Command{
		Use:   "pick",
		Short: "Pick an agent or pane",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			client := tmux.NewRealClient("")
			return session.Pick(resolvedStateDir, client, session.PickOpts{
				Emit:   emit,
				Insert: insert,
			})
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&emit, "emit", "alias", "what to output: alias, name, pane-id")
	cmd.Flags().StringVar(&insert, "insert", "raw", "insertion mode: send-target, raw")
	return cmd
}

func newListPanesCmd() *cobra.Command {
	var configPath string
	var stateDir string

	cmd := &cobra.Command{
		Use:    "__list-panes",
		Short:  "List panes for fzf picker",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			client := tmux.NewRealClient("")
			output, err := session.ListPanes(resolvedStateDir, client, session.ListPanesOpts{})
			if err != nil {
				return err
			}

			fmt.Println(output)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	return cmd
}

func newPreviewPaneCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var paneID string
	var alias string

	cmd := &cobra.Command{
		Use:    "__preview-pane",
		Short:  "Preview a pane for fzf picker",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			client := tmux.NewRealClient("")
			output, err := session.PreviewPane(resolvedStateDir, client, session.PreviewPaneOpts{
				PaneID: paneID,
				Alias:  alias,
			})
			if err != nil {
				return err
			}

			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&paneID, "pane", "", "pane ID to preview")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias to preview")
	return cmd
}

func resolveStateDir(configPath, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	if stateDir := strings.TrimSpace(os.Getenv("TMUXICATE_STATE_DIR")); stateDir != "" {
		return stateDir, nil
	}
	if strings.TrimSpace(configPath) == "" {
		return "", errors.New("state dir is required")
	}

	cfg, err := config.LoadResolved(configPath)
	if err != nil {
		return "", err
	}
	return cfg.Session.StateDir, nil
}

func resolveAgent(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	if agent := strings.TrimSpace(os.Getenv("TMUXICATE_AGENT")); agent != "" {
		return agent, nil
	}
	return "", errors.New("agent is required")
}

func readReplyBody(bodyFile string, useStdin bool) ([]byte, error) {
	if strings.TrimSpace(bodyFile) != "" {
		return os.ReadFile(bodyFile)
	}

	stdinHasData := false
	if info, err := os.Stdin.Stat(); err == nil {
		stdinHasData = info.Mode()&os.ModeCharDevice == 0
	}

	if useStdin || stdinHasData {
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			return nil, errors.New("reply body is empty")
		}
		return body, nil
	}

	return nil, errors.New("reply body required via --body-file or stdin")
}

func printReadResult(result *session.ReadResult) {
	attachments := "-"
	if len(result.Attachments) > 0 {
		parts := make([]string, 0, len(result.Attachments))
		for _, attachment := range result.Attachments {
			parts = append(parts, fmt.Sprintf("%s (%s)", attachment.Path, attachment.MediaType))
		}
		attachments = strings.Join(parts, ", ")
	}

	subject := result.Subject
	if subject == "" {
		subject = "-"
	}

	fmt.Printf("Message: %s\n", result.MessageID)
	fmt.Printf("Seq: %d\n", result.Seq)
	fmt.Printf("Thread: %s\n", result.Thread)
	fmt.Printf("From: %s\n", result.From)
	fmt.Printf("To: %s\n", strings.Join(agentNames(result.To), ", "))
	fmt.Printf("Kind: %s\n", result.Kind)
	fmt.Printf("Priority: %s\n", result.Priority)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("Created: %s\n", result.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Requires-Claim: %t\n", result.RequiresClaim)
	fmt.Printf("Attachments: %s\n", attachments)
	fmt.Printf("\n--- body.md ---\n%s", result.Body)
	if !strings.HasSuffix(result.Body, "\n") {
		fmt.Println()
	}
}

func agentNames(names []protocol.AgentName) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, string(name))
	}
	return out
}

func formatAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	if age < time.Minute {
		secs := int(age / time.Second)
		if secs == 0 {
			secs = 1
		}
		return fmt.Sprintf("%ds", secs)
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int(age/time.Minute))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%dh", int(age/time.Hour))
	}
	return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
}

func printStatusReport(report *session.StatusReport) {
	uptime := "-"
	if report.Uptime > 0 {
		uptime = formatAge(report.Uptime)
	}
	daemonState := "unhealthy"
	if report.DaemonHealthy {
		daemonState = "healthy"
	}

	fmt.Printf("Session: %s   State: %s   Uptime: %s   Daemon: %s\n", report.SessionName, report.State, uptime, daemonState)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tPANE\tOBSERVED\tDECLARED\tUNREAD\tACTIVE\tLAST-EVENT")
	for _, agent := range report.AgentStatuses {
		lastEvent := "-"
		if agent.LastEvent != nil {
			lastEvent = formatAge(time.Since(*agent.LastEvent))
		}
		fmt.Fprintf(w, "%s(%s)\t%s\t%s\t%s\t%d\t%d\t%s\n",
			agent.Name,
			agent.Alias,
			agent.PaneID,
			agent.ObservedState,
			agent.DeclaredState,
			agent.UnreadCount,
			agent.ActiveCount,
			lastEvent,
		)
	}
	_ = w.Flush()

	fmt.Println()
	fmt.Printf("FLOW\nsent=%d  acked=%d  done=%d  pending=%d  retrying=%d  failed=%d\n",
		report.FlowStats.Sent,
		report.FlowStats.Acked,
		report.FlowStats.Done,
		report.FlowStats.Pending,
		report.FlowStats.Retrying,
		report.FlowStats.Failed,
	)

	fmt.Println()
	fmt.Printf("THREADS\nopen=%d  resolved=%d  closed=%d\n",
		report.ThreadStats.Open,
		report.ThreadStats.Resolved,
		report.ThreadStats.Closed,
	)
}

func filepathJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathSeparator))
}

func stubRun(_ *cobra.Command, _ []string) {
	fmt.Println("not implemented yet")
}
