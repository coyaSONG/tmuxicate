package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const listPaneFieldSep = "\x1f"

type RealClient struct {
	binary string
}

func NewRealClient(binary string) *RealClient {
	if binary == "" {
		binary = "tmux"
	}

	return &RealClient{binary: binary}
}

var _ Client = (*RealClient)(nil)

func (c *RealClient) NewSession(ctx context.Context, spec SessionSpec) (string, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return "", errors.New("session name is required")
	}

	args := []string{"new-session", "-d", "-P", "-F", "#{pane_id}", "-s", spec.Name}
	if spec.WindowName != "" {
		args = append(args, "-n", spec.WindowName)
	}
	if spec.StartDirectory != "" {
		args = append(args, "-c", spec.StartDirectory)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func (c *RealClient) SplitPane(ctx context.Context, spec SplitSpec) (string, error) {
	if strings.TrimSpace(spec.TargetPane) == "" {
		return "", errors.New("target pane is required")
	}

	args := []string{"split-window", "-P", "-F", "#{pane_id}", "-t", spec.TargetPane}
	switch spec.Direction {
	case "h":
		args = append(args, "-h")
	case "v":
		args = append(args, "-v")
	case "":
		// tmux default
	default:
		return "", fmt.Errorf("invalid split direction %q", spec.Direction)
	}

	if spec.Percentage > 0 {
		args = append(args, "-p", strconv.Itoa(spec.Percentage))
	}
	if spec.StartDirectory != "" {
		args = append(args, "-c", spec.StartDirectory)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func (c *RealClient) SendKeys(ctx context.Context, paneID string, text string, enter bool) error {
	if strings.TrimSpace(paneID) == "" {
		return errors.New("pane id is required")
	}

	if text != "" {
		if _, err := c.run(ctx, "send-keys", "-t", paneID, "-l", text); err != nil {
			return err
		}
	}
	if enter {
		if _, err := c.run(ctx, "send-keys", "-t", paneID, "Enter"); err != nil {
			return err
		}
	}

	return nil
}

func (c *RealClient) CapturePane(ctx context.Context, paneID string, lines int) (string, error) {
	if strings.TrimSpace(paneID) == "" {
		return "", errors.New("pane id is required")
	}
	if lines <= 0 {
		lines = 100
	}

	return c.run(ctx, "capture-pane", "-pJ", "-t", paneID, "-S", fmt.Sprintf("-%d", lines))
}

func (c *RealClient) SetBuffer(ctx context.Context, data string) error {
	_, err := c.run(ctx, "set-buffer", "--", data)
	return err
}

func (c *RealClient) PasteBuffer(ctx context.Context, paneID string) error {
	if strings.TrimSpace(paneID) == "" {
		return errors.New("pane id is required")
	}

	_, err := c.run(ctx, "paste-buffer", "-p", "-t", paneID)
	return err
}

func (c *RealClient) PipePane(ctx context.Context, paneID string, cmd string) error {
	if strings.TrimSpace(paneID) == "" {
		return errors.New("pane id is required")
	}
	if strings.TrimSpace(cmd) == "" {
		return errors.New("pipe command is required")
	}

	_, err := c.run(ctx, "pipe-pane", "-o", "-t", paneID, cmd)
	return err
}

func (c *RealClient) SetPaneOption(ctx context.Context, paneID, key, value string) error {
	if paneID == "" || key == "" {
		return errors.New("pane id and key are required")
	}

	_, err := c.run(ctx, "set-option", "-p", "-t", paneID, key, value)
	return err
}

func (c *RealClient) ShowPaneOption(ctx context.Context, paneID, key string) (string, error) {
	if paneID == "" || key == "" {
		return "", errors.New("pane id and key are required")
	}

	out, err := c.run(ctx, "show-options", "-pv", "-t", paneID, key)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func (c *RealClient) SetSessionOption(ctx context.Context, session, key, value string) error {
	if session == "" || key == "" {
		return errors.New("session and key are required")
	}

	_, err := c.run(ctx, "set-option", "-t", session, key, value)
	return err
}

func (c *RealClient) ListPanes(ctx context.Context, session string) ([]PaneInfo, error) {
	if strings.TrimSpace(session) == "" {
		return nil, errors.New("session is required")
	}

	format := strings.Join([]string{
		"#{pane_id}",
		"#{pane_title}",
		"#{session_name}",
		"#{window_name}",
		"#{window_id}",
		"#{pane_index}",
		"#{window_index}",
		"#{pane_current_command}",
		"#{?pane_active,1,0}",
	}, listPaneFieldSep)

	out, err := c.run(ctx, "list-panes", "-t", session, "-F", format)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	panes := make([]PaneInfo, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, listPaneFieldSep)
		if len(fields) != 9 {
			return nil, fmt.Errorf("unexpected list-panes output: %q", line)
		}

		paneIndex, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("parse pane index: %w", err)
		}
		windowIndex, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, fmt.Errorf("parse window index: %w", err)
		}

		panes = append(panes, PaneInfo{
			PaneID:         fields[0],
			PaneTitle:      fields[1],
			SessionName:    fields[2],
			WindowName:     fields[3],
			WindowID:       fields[4],
			PaneIndex:      paneIndex,
			WindowIndex:    windowIndex,
			CurrentCommand: fields[7],
			Active:         fields[8] == "1",
		})
	}

	return panes, nil
}

func (c *RealClient) HasSession(ctx context.Context, session string) (bool, error) {
	if strings.TrimSpace(session) == "" {
		return false, errors.New("session is required")
	}

	_, err := c.run(ctx, "has-session", "-t", session)
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, err
}

func (c *RealClient) KillSession(ctx context.Context, session string) error {
	if strings.TrimSpace(session) == "" {
		return errors.New("session is required")
	}

	_, err := c.run(ctx, "kill-session", "-t", session)
	return err
}

func (c *RealClient) SelectLayout(ctx context.Context, window, layout string) error {
	if window == "" || layout == "" {
		return errors.New("window and layout are required")
	}

	_, err := c.run(ctx, "select-layout", "-t", window, layout)
	return err
}

func (c *RealClient) DisplayPopup(ctx context.Context, spec *PopupSpec) error {
	args := []string{"display-popup", "-E"}
	if spec.TargetPane != "" {
		args = append(args, "-t", spec.TargetPane)
	}
	if spec.Title != "" {
		args = append(args, "-T", spec.Title)
	}
	if spec.Width != "" {
		args = append(args, "-w", spec.Width)
	}
	if spec.Height != "" {
		args = append(args, "-h", spec.Height)
	}
	if spec.StartDirectory != "" {
		args = append(args, "-d", spec.StartDirectory)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}

	_, err := c.run(ctx, args...)
	return err
}

func (c *RealClient) SetPaneTitle(ctx context.Context, paneID, title string) error {
	if paneID == "" {
		return errors.New("pane id is required")
	}

	_, err := c.run(ctx, "select-pane", "-t", paneID, "-T", title)
	return err
}

func (c *RealClient) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s %s failed: %w: %s", c.binary, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%s %s failed: %w", c.binary, strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}
