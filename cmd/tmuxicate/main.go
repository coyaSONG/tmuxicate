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
		newTargetCmd(),
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
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "route-task",
		Short: "Route a child task to one deterministic owner",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadResolved(configPath)
			if err != nil {
				return err
			}

			req := protocol.RouteChildTaskRequest{
				RunID:          protocol.RunID(runID),
				TaskClass:      protocol.TaskClass(taskClass),
				Domains:        domains,
				Goal:           goal,
				ExpectedOutput: expectedOutput,
				ReviewRequired: reviewRequired,
				OwnerOverride:  protocol.AgentName(ownerOverride),
				OverrideReason: overrideReason,
			}

			if dryRun {
				preview, err := session.PreviewRouteChildTask(cfg, req)
				if err != nil {
					return err
				}
				return printRouteTaskSelection(cmd.OutOrStdout(), "", preview.SelectedOwner, preview.Placement, preview.Decision, true)
			}

			task, decision, err := session.RouteChildTask(cfg, mailbox.NewStore(cfg.Session.StateDir), req)
			if err != nil {
				return err
			}

			return printRouteTaskSelection(cmd.OutOrStdout(), string(task.TaskID), task.Owner, task.Placement, decision, false)
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview owner and execution target without creating task artifacts")
	_ = cmd.MarkFlagRequired("run")
	_ = cmd.MarkFlagRequired("task-class")
	_ = cmd.MarkFlagRequired("domain")
	_ = cmd.MarkFlagRequired("goal")
	_ = cmd.MarkFlagRequired("expected-output")
	return cmd
}

func printRouteTaskSelection(out io.Writer, taskID string, owner protocol.AgentName, placement *protocol.TaskPlacement, decision *protocol.RoutingDecision, previewOnly bool) error {
	if previewOnly {
		if _, err := fmt.Fprintln(out, "Preview Only: true"); err != nil {
			return err
		}
	} else if strings.TrimSpace(taskID) != "" {
		if _, err := fmt.Fprintln(out, taskID); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "Selected Owner: %s\n", owner); err != nil {
		return err
	}
	if placement != nil {
		if _, err := fmt.Fprintf(out, "Execution Target: %s\n", placement.Target.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Target Kind: %s\n", placement.Target.Kind); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Target Capabilities: %s\n", formatExecutionTargetCapabilities(placement.Target.Capabilities)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Placement Reason: %s\n", placement.Reason); err != nil {
			return err
		}
	}
	if decision != nil && decision.Adaptive != nil && decision.Adaptive.Applied {
		if _, err := fmt.Fprintf(out, "Adaptive Routing: %s\n", decision.Adaptive.Reason); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Adaptive Baseline: %s\n", decision.Adaptive.BaselineOwner); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Adaptive Score: historical=%d manual=%d total=%d\n", decision.Adaptive.HistoricalScore, decision.Adaptive.ManualWeight, decision.Adaptive.TotalScore); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Adaptive Evidence: %s\n", formatAdaptiveEvidence(decision.Adaptive.Evidence)); err != nil {
			return err
		}
	}
	if decision != nil && len(decision.ExcludedTargets) > 0 {
		if _, err := fmt.Fprintf(out, "Excluded Targets: %s\n", formatExcludedTargets(decision.ExcludedTargets)); err != nil {
			return err
		}
	}

	return nil
}

func formatExecutionTargetCapabilities(capabilities []string) string {
	if len(capabilities) == 0 {
		return "-"
	}

	return strings.Join(capabilities, ", ")
}

func formatExcludedTargets(exclusions []protocol.RouteTargetExclusion) string {
	if len(exclusions) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(exclusions))
	for _, exclusion := range exclusions {
		parts = append(parts, fmt.Sprintf("%s/%s (%s)", exclusion.Owner, exclusion.TargetName, exclusion.Status))
	}
	return strings.Join(parts, ", ")
}

func formatAdaptiveEvidence(evidence []protocol.AdaptiveRoutingEvidenceRef) string {
	parts := make([]string, 0, len(evidence))
	for _, ref := range evidence {
		parts = append(parts, fmt.Sprintf("run=%s task=%s status=%s", ref.RunID, ref.SourceTaskID, ref.Status))
	}
	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, "; ")
}

func newRunShowCmd() *cobra.Command {
	var configPath string
	var timeline bool
	var timelineOnly bool
	var timelineOwner string
	var timelineState string
	var timelineClass string
	var timelineTarget string

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

			filter := session.RunTimelineFilter{
				Owner:           timelineOwner,
				State:           timelineState,
				TaskClass:       protocol.TaskClass(timelineClass),
				ExecutionTarget: timelineTarget,
			}
			if timelineClass != "" {
				if err := filter.TaskClass.Validate(); err != nil {
					return fmt.Errorf("timeline-class: %w", err)
				}
			}

			output, err := session.FormatRunGraphView(cfg.Session.StateDir, graph, session.RunGraphFormatOptions{
				Timeline:       timeline,
				TimelineOnly:   timelineOnly,
				TimelineFilter: filter,
			})
			if err != nil {
				return err
			}
			if !strings.HasPrefix(output, "Run: ") {
				return fmt.Errorf("run show output must start with Run: header")
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output)
			return err
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().BoolVar(&timeline, "timeline", false, "include the derived run timeline")
	cmd.Flags().BoolVar(&timelineOnly, "timeline-only", false, "show only the run header, summary, and timeline")
	cmd.Flags().StringVar(&timelineOwner, "timeline-owner", "", "filter timeline rows by owner")
	cmd.Flags().StringVar(&timelineState, "timeline-state", "", "filter timeline rows by state")
	cmd.Flags().StringVar(&timelineClass, "timeline-class", "", "filter timeline rows by task class")
	cmd.Flags().StringVar(&timelineTarget, "timeline-target", "", "filter timeline rows by execution target")
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
	var taskClass string
	var domains []string
	var goal string
	var expectedOutput string

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
			opts := session.BlockerResolveOpts{
				RunID:          protocol.RunID(args[0]),
				SourceTaskID:   protocol.TaskID(args[1]),
				Action:         resolutionAction,
				Owner:          owner,
				Reason:         reason,
				Body:           body,
				TaskClass:      protocol.TaskClass(taskClass),
				Domains:        domains,
				Goal:           goal,
				ExpectedOutput: expectedOutput,
			}
			if err := opts.Validate(); err != nil {
				return err
			}

			store := mailbox.NewStore(resolvedStateDir)
			if err := session.BlockerResolve(resolvedStateDir, store, opts); err != nil {
				return err
			}

			fmt.Println("resolved")
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "override session state directory")
	cmd.Flags().StringVar(&action, "action", "", "blocker resolution action: manual_reroute, partial_replan, clarify, or dismiss")
	cmd.Flags().StringVar(&owner, "owner", "", "override reroute owner for manual_reroute or partial_replan")
	cmd.Flags().StringVar(&reason, "reason", "", "operator resolution reason")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to clarification body file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read clarification body from stdin")
	cmd.Flags().StringVar(&taskClass, "task-class", "", "replacement task class for partial_replan")
	cmd.Flags().StringSliceVar(&domains, "domains", nil, "replacement task domains for partial_replan")
	cmd.Flags().StringVar(&goal, "goal", "", "replacement task goal for partial_replan")
	cmd.Flags().StringVar(&expectedOutput, "expected-output", "", "replacement task expected output for partial_replan")
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
		RunE: func(cmd *cobra.Command, args []string) error {
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

			summaryOutput := ""
			env, _, err := mailbox.NewStore(resolvedStateDir).ReadMessage(protocol.MessageID(args[0]))
			if err != nil {
				return fmt.Errorf("read completed task message: %w", err)
			}
			runID := strings.TrimSpace(env.Meta["run_id"])
			rootMessageID := strings.TrimSpace(env.Meta["root_message_id"])
			if runID != "" && rootMessageID == args[0] {
				graph, err := session.LoadRunGraph(resolvedStateDir, protocol.RunID(runID))
				if err != nil {
					return err
				}
				summaryOutput = session.FormatRunSummary(session.BuildRunSummary(graph))

				cfg, err := config.LoadResolved(filepathJoin(resolvedStateDir, "config.resolved.yaml"))
				if err != nil {
					return err
				}
				preferences, err := session.BuildAdaptiveRoutingPreferences(cfg, resolvedStateDir, graph.Run.Coordinator)
				if err != nil {
					return err
				}
				if err := mailbox.NewCoordinatorStore(resolvedStateDir).WriteAdaptiveRoutingPreferences(preferences); err != nil {
					return err
				}
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "done"); err != nil {
				return err
			}
			if summaryOutput != "" {
				_, err = fmt.Fprint(cmd.OutOrStdout(), summaryOutput)
				return err
			}
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

func newTargetCmd() *cobra.Command {
	var configPath string
	var stateDir string

	cmd := &cobra.Command{
		Use:   "target",
		Short: "Inspect and control execution targets",
	}

	resolveStateDir := func() (string, error) {
		if strings.TrimSpace(stateDir) != "" {
			return stateDir, nil
		}
		cfg, err := config.LoadResolved(configPath)
		if err != nil {
			return "", err
		}
		return cfg.Session.StateDir, nil
	}

	cmd.PersistentFlags().StringVar(&configPath, "config", "tmuxicate.yaml", "path to tmuxicate config")
	cmd.PersistentFlags().StringVar(&stateDir, "state-dir", "", "override session state directory")

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List execution target health and dispatch status",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedStateDir, err := resolveStateDir()
			if err != nil {
				return err
			}
			statuses, err := session.ListTargetStatuses(resolvedStateDir)
			if err != nil {
				return err
			}
			printTargetStatuses(statuses)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status <target>",
		Short: "Show detailed status for one execution target",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir()
			if err != nil {
				return err
			}
			statuses, err := session.ListTargetStatuses(resolvedStateDir)
			if err != nil {
				return err
			}
			for _, status := range statuses {
				if status.Name == args[0] {
					printTargetStatus(status)
					return nil
				}
			}
			return fmt.Errorf("unknown target %q", args[0])
		},
	})

	var heartbeatStatus string
	var heartbeatSummary string
	var heartbeatCapabilities []string
	heartbeatCmd := &cobra.Command{
		Use:   "heartbeat <target>",
		Short: "Record target health from a remote launcher or worker",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir()
			if err != nil {
				return err
			}
			status, err := parseTargetAvailability(heartbeatStatus)
			if err != nil {
				return err
			}
			report, err := session.TargetHeartbeat(resolvedStateDir, args[0], status, heartbeatSummary, heartbeatCapabilities)
			if err != nil {
				return err
			}
			printTargetStatus(*report)
			return nil
		},
	}
	heartbeatCmd.Flags().StringVar(&heartbeatStatus, "status", string(mailbox.TargetAvailabilityReady), "target availability: ready, degraded, offline, disabled, unknown")
	heartbeatCmd.Flags().StringVar(&heartbeatSummary, "summary", "", "health summary")
	heartbeatCmd.Flags().StringSliceVar(&heartbeatCapabilities, "capability", nil, "capability override reported by the worker")
	cmd.AddCommand(heartbeatCmd)

	var disableReason string
	disableCmd := &cobra.Command{
		Use:   "disable <target>",
		Short: "Disable an execution target for future routing",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir()
			if err != nil {
				return err
			}
			report, err := session.DisableTarget(resolvedStateDir, args[0], disableReason)
			if err != nil {
				return err
			}
			printTargetStatus(*report)
			return nil
		},
	}
	disableCmd.Flags().StringVar(&disableReason, "reason", "", "operator reason for disabling the target")
	cmd.AddCommand(disableCmd)

	var enableReason string
	enableCmd := &cobra.Command{
		Use:   "enable <target>",
		Short: "Enable an execution target and redispatch pending work",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resolvedStateDir, err := resolveStateDir()
			if err != nil {
				return err
			}
			report, redispatched, err := session.EnableTarget(resolvedStateDir, args[0], enableReason)
			if err != nil {
				return err
			}
			printTargetStatus(*report)
			fmt.Printf("Redispatched: %d\n", redispatched)
			return nil
		},
	}
	enableCmd.Flags().StringVar(&enableReason, "reason", "", "operator reason for re-enabling the target")
	cmd.AddCommand(enableCmd)

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

	if len(report.TargetStatuses) > 0 {
		fmt.Println()
		fmt.Println("TARGET")
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tKIND\tAVAILABILITY\tPENDING\tFAILED\tLAST-DISPATCH\tSUMMARY")
		for _, target := range report.TargetStatuses {
			lastDispatch := "-"
			if target.LastDispatch != nil {
				lastDispatch = formatAge(time.Since(*target.LastDispatch))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
				target.Name,
				target.Kind,
				target.Availability,
				target.PendingDispatches,
				target.FailedDispatches,
				lastDispatch,
				target.Summary,
			)
		}
		_ = tw.Flush()
	}

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

func parseTargetAvailability(raw string) (mailbox.TargetAvailability, error) {
	switch mailbox.TargetAvailability(strings.TrimSpace(raw)) {
	case mailbox.TargetAvailabilityReady, mailbox.TargetAvailabilityDegraded, mailbox.TargetAvailabilityOffline, mailbox.TargetAvailabilityDisabled, mailbox.TargetAvailabilityUnknown:
		return mailbox.TargetAvailability(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("invalid target availability %q", raw)
	}
}

func printTargetStatuses(statuses []session.TargetStatus) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tAVAILABILITY\tPENDING\tFAILED\tLAST-HEARTBEAT\tLAST-DISPATCH\tSUMMARY")
	for _, status := range statuses {
		lastHeartbeat := "-"
		if status.LastHeartbeat != nil {
			lastHeartbeat = formatAge(time.Since(*status.LastHeartbeat))
		}
		lastDispatch := "-"
		if status.LastDispatch != nil {
			lastDispatch = formatAge(time.Since(*status.LastDispatch))
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			status.Name,
			status.Kind,
			status.Availability,
			status.PendingDispatches,
			status.FailedDispatches,
			lastHeartbeat,
			lastDispatch,
			status.Summary,
		)
	}
	_ = tw.Flush()
}

func printTargetStatus(status session.TargetStatus) {
	fmt.Printf("Target: %s\n", status.Name)
	fmt.Printf("Kind: %s\n", status.Kind)
	fmt.Printf("Availability: %s\n", status.Availability)
	fmt.Printf("Pane-Backed: %t\n", status.PaneBacked)
	fmt.Printf("Pending Dispatches: %d\n", status.PendingDispatches)
	fmt.Printf("Failed Dispatches: %d\n", status.FailedDispatches)
	fmt.Printf("Summary: %s\n", status.Summary)
	if status.Source != "" {
		fmt.Printf("Source: %s\n", status.Source)
	}
	if status.DisabledReason != "" {
		fmt.Printf("Disabled Reason: %s\n", status.DisabledReason)
	}
	if status.LastError != "" {
		fmt.Printf("Last Error: %s\n", status.LastError)
	}
	if status.LastHeartbeat != nil {
		fmt.Printf("Last Heartbeat: %s\n", status.LastHeartbeat.Format(time.RFC3339))
	}
	if status.LastDispatch != nil {
		fmt.Printf("Last Dispatch: %s\n", status.LastDispatch.Format(time.RFC3339))
	}
}

func stubRun(_ *cobra.Command, _ []string) {
	fmt.Println("not implemented yet")
}
