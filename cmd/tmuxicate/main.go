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
		newRunCmd(),
		newBlockerCmd(),
		newReviewCmd(),
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
	var sendKind string

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
				Kind:    protocol.Kind(sendKind),
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
	cmd.Flags().StringVar(&sendKind, "kind", string(protocol.KindTask), "message kind")
	return cmd
}

func newRunCmd() *cobra.Command {
	var configPath string
	var coordinator string

	cmd := &cobra.Command{
		Use:   "run <goal...>",
		Short: "Start or manage a coordinator run",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			run, err := session.Run(cfg, mailbox.NewStore(cfg.Session.StateDir), session.RunRequest{
				Goal:        strings.Join(args, " "),
				Coordinator: coordinator,
				CreatedBy:   "human",
			})
			if err != nil {
				return err
			}

			fmt.Println(run.RunID)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&coordinator, "coordinator", "", "coordinator agent name or alias")
	_ = cmd.MarkFlagRequired("coordinator")
	cmd.AddCommand(newRunAddTaskCmd())
	cmd.AddCommand(newRunRouteTaskCmd())
	cmd.AddCommand(newRunShowCmd())
	return cmd
}

func newRunAddTaskCmd() *cobra.Command {
	var configPath string
	var runID string
	var owner string
	var goal string
	var expectedOutput string
	var dependsOn []string
	var reviewRequired bool

	cmd := &cobra.Command{
		Use:   "add-task",
		Short: "Add a child task to a coordinator run",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			dependencyIDs := make([]protocol.TaskID, 0, len(dependsOn))
			for _, dep := range dependsOn {
				dependencyIDs = append(dependencyIDs, protocol.TaskID(dep))
			}

			task, err := session.AddChildTask(cfg, mailbox.NewStore(cfg.Session.StateDir), session.ChildTaskRequest{
				ParentRunID:    protocol.RunID(runID),
				Owner:          owner,
				Goal:           goal,
				ExpectedOutput: expectedOutput,
				DependsOn:      dependencyIDs,
				ReviewRequired: reviewRequired,
			})
			if err != nil {
				return err
			}

			fmt.Println(task.TaskID)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&runID, "run", "", "run identifier")
	cmd.Flags().StringVar(&owner, "owner", "", "task owner agent name or alias")
	cmd.Flags().StringVar(&goal, "goal", "", "task goal")
	cmd.Flags().StringVar(&expectedOutput, "expected-output", "", "expected task output")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "task dependencies")
	cmd.Flags().BoolVar(&reviewRequired, "review-required", false, "mark the child task as requiring review")
	_ = cmd.MarkFlagRequired("run")
	_ = cmd.MarkFlagRequired("owner")
	_ = cmd.MarkFlagRequired("goal")
	_ = cmd.MarkFlagRequired("expected-output")
	return cmd
}

func newRunRouteTaskCmd() *cobra.Command {
	var configPath string
	var runID string
	var taskClass string
	var domains []string
	var goal string
	var expectedOutput string
	var reviewRequired bool
	var ownerOverride string
	var overrideReason string

	cmd := &cobra.Command{
		Use:   "route-task",
		Short: "Route a child task to one deterministic owner",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			task, _, err := session.RouteChildTask(cfg, mailbox.NewStore(cfg.Session.StateDir), protocol.RouteChildTaskRequest{
				RunID:          protocol.RunID(runID),
				TaskClass:      protocol.TaskClass(taskClass),
				Domains:        domains,
				Goal:           goal,
				ExpectedOutput: expectedOutput,
				ReviewRequired: reviewRequired,
				OwnerOverride:  protocol.AgentName(ownerOverride),
				OverrideReason: overrideReason,
			})
			if err != nil {
				return err
			}

			fmt.Println(task.TaskID)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&runID, "run", "", "run identifier")
	cmd.Flags().StringVar(&taskClass, "task-class", "", "routing task class")
	cmd.Flags().StringSliceVar(&domains, "domain", nil, "required task domain (repeat for multiple domains)")
	cmd.Flags().StringVar(&goal, "goal", "", "task goal")
	cmd.Flags().StringVar(&expectedOutput, "expected-output", "", "expected task output")
	cmd.Flags().BoolVar(&reviewRequired, "review-required", false, "mark the child task as requiring review")
	cmd.Flags().StringVar(&ownerOverride, "owner-override", "", "explicit owner override after routing review")
	cmd.Flags().StringVar(&overrideReason, "override-reason", "", "reason for overriding routed owner selection")
	_ = cmd.MarkFlagRequired("run")
	_ = cmd.MarkFlagRequired("task-class")
	_ = cmd.MarkFlagRequired("domain")
	_ = cmd.MarkFlagRequired("goal")
	_ = cmd.MarkFlagRequired("expected-output")
	return cmd
}

func newRunShowCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show a coordinator run from durable disk artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			graph, err := session.LoadRunGraph(cfg.Session.StateDir, protocol.RunID(args[0]))
			if err != nil {
				return err
			}

			output := session.FormatRunGraph(graph)
			if !strings.HasPrefix(output, "Run: ") {
				return fmt.Errorf("run show output must start with Run: header")
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output)
			return err
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
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

func newBlockerCmd() *cobra.Command {
	blockerCmd := &cobra.Command{
		Use:   "blocker",
		Short: "Manage blocker workflows",
		Run:   stubRun,
	}

	blockerCmd.AddCommand(newBlockerResolveCmd())

	return blockerCmd
}

func newReviewCmd() *cobra.Command {
	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Manage review workflows",
		Run:   stubRun,
	}

	reviewCmd.AddCommand(newReviewRespondCmd())

	return reviewCmd
}

func newBlockerResolveCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var action string
	var owner string
	var reason string
	var bodyFile string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "resolve <run-id> <source-task-id>",
		Short: "Resolve an escalated blocker case",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir(configPath, stateDir)
			if err != nil {
				return err
			}

			resolutionAction := protocol.BlockerResolutionAction(action)
			if err := resolutionAction.Validate(); err != nil {
				return fmt.Errorf("invalid action: %w", err)
			}

			body, err := readOptionalReplyBody(bodyFile, useStdin)
			if err != nil {
				return err
			}

			store := mailbox.NewStore(resolvedStateDir)
			if err := session.BlockerResolve(
				resolvedStateDir,
				store,
				protocol.RunID(args[0]),
				protocol.TaskID(args[1]),
				resolutionAction,
				owner,
				reason,
				body,
			); err != nil {
				return err
			}

			fmt.Println("resolved")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&action, "action", "", "blocker resolution action: manual_reroute, clarify, or dismiss")
	cmd.Flags().StringVar(&owner, "owner", "", "override reroute owner for manual_reroute")
	cmd.Flags().StringVar(&reason, "reason", "", "operator resolution reason")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to clarification body file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read clarification body from stdin")
	_ = cmd.MarkFlagRequired("action")

	return cmd
}

func newReviewRespondCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var outcome string
	var bodyFile string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "respond <review-message-id>",
		Short: "Respond to a review request",
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
			reviewOutcome := protocol.ReviewOutcome(outcome)
			if err := reviewOutcome.Validate(); err != nil {
				return fmt.Errorf("invalid outcome: %w", err)
			}

			body, err := readReplyBody(bodyFile, useStdin)
			if err != nil {
				return err
			}

			store := mailbox.NewStore(resolvedStateDir)
			msgID, err := session.ReviewRespond(resolvedStateDir, store, resolvedAgent, protocol.MessageID(args[0]), reviewOutcome, body)
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
	cmd.Flags().StringVar(&outcome, "outcome", "", "review outcome: approved or changes_requested")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to review response body file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read review response body from stdin")
	_ = cmd.MarkFlagRequired("outcome")

	return cmd
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
	var kind string
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

			waitKind := protocol.WaitKind(kind)
			if err := waitKind.Validate(); err != nil {
				return fmt.Errorf("invalid kind: %w", err)
			}

			if err := session.TaskWait(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]), waitKind, on, reason); err != nil {
				return err
			}
			fmt.Println("waiting")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&kind, "kind", "", "wait kind: dependency_reply or external_event")
	cmd.Flags().StringVar(&on, "on", "", "target agent or dependency being waited on")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for waiting")
	_ = cmd.MarkFlagRequired("kind")
	return cmd
}

func newTaskBlockCmd() *cobra.Command {
	var configPath string
	var stateDir string
	var agent string
	var kind string
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

			blockKind := protocol.BlockKind(kind)
			if err := blockKind.Validate(); err != nil {
				return fmt.Errorf("invalid kind: %w", err)
			}

			if err := session.TaskBlock(resolvedStateDir, resolvedAgent, protocol.MessageID(args[0]), blockKind, on, reason); err != nil {
				return err
			}
			fmt.Println("blocked")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&agent, "agent", "", "current agent name or alias")
	cmd.Flags().StringVar(&kind, "kind", "", "block kind: agent_clarification, reroute_needed, human_decision, or unsupported")
	cmd.Flags().StringVar(&on, "on", "human", "target agent or dependency causing the block")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for the block")
	_ = cmd.MarkFlagRequired("kind")
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

func readOptionalReplyBody(bodyFile string, useStdin bool) ([]byte, error) {
	if strings.TrimSpace(bodyFile) == "" && !useStdin {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return nil, nil
		}
	}

	return readReplyBody(bodyFile, useStdin)
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
	for i := range report.AgentStatuses {
		agent := &report.AgentStatuses[i]
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
