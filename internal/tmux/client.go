package tmux

import "context"

type Client interface {
	NewSession(ctx context.Context, spec SessionSpec) (paneID string, err error)
	SplitPane(ctx context.Context, spec SplitSpec) (paneID string, err error)
	SendKeys(ctx context.Context, paneID string, text string, enter bool) error
	CapturePane(ctx context.Context, paneID string, lines int) (string, error)
	SetBuffer(ctx context.Context, data string) error
	PasteBuffer(ctx context.Context, paneID string) error
	PipePane(ctx context.Context, paneID string, cmd string) error
	SetPaneOption(ctx context.Context, paneID, key, value string) error
	ShowPaneOption(ctx context.Context, paneID, key string) (string, error)
	SetSessionOption(ctx context.Context, session, key, value string) error
	ListPanes(ctx context.Context, session string) ([]PaneInfo, error)
	HasSession(ctx context.Context, session string) (bool, error)
	KillSession(ctx context.Context, session string) error
	SelectLayout(ctx context.Context, window, layout string) error
	DisplayPopup(ctx context.Context, spec PopupSpec) error
	SetPaneTitle(ctx context.Context, paneID, title string) error
}

type SessionSpec struct {
	Name           string
	WindowName     string
	StartDirectory string
	Command        string
}

type SplitSpec struct {
	TargetPane     string
	Direction      string
	Percentage     int
	StartDirectory string
	Command        string
}

type PopupSpec struct {
	TargetPane     string
	Title          string
	Width          string
	Height         string
	StartDirectory string
	Command        string
}

type PaneInfo struct {
	PaneID         string
	PaneTitle      string
	SessionName    string
	WindowName     string
	WindowID       string
	PaneIndex      int
	WindowIndex    int
	CurrentCommand string
	Active         bool
}
