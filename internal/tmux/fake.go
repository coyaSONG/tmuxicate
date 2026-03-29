package tmux

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type FakeClient struct {
	Mu sync.Mutex

	NewSessionCalls       []SessionSpec
	SplitPaneCalls        []SplitSpec
	SendKeysCalls         []SendKeysCall
	CapturePaneCalls      []CapturePaneCall
	SetBufferCalls        []string
	PasteBufferCalls      []string
	PipePaneCalls         []PipePaneCall
	SetPaneOptionCalls    []SetPaneOptionCall
	SetSessionOptionCalls []SetSessionOptionCall
	SelectLayoutCalls     []SelectLayoutCall
	DisplayPopupCalls     []PopupSpec
	SetPaneTitleCalls     []SetPaneTitleCall
	KillSessionCalls      []string

	NextPaneID     int
	Sessions       map[string]bool
	PaneOptions    map[string]map[string]string
	SessionOptions map[string]map[string]string
	PaneCaptures   map[string]string
	PanesBySession map[string][]PaneInfo
	PopupErr       error
	Err            error
	Buffer         string
}

type SendKeysCall struct {
	PaneID string
	Text   string
	Enter  bool
}

type CapturePaneCall struct {
	PaneID string
	Lines  int
}

type PipePaneCall struct {
	PaneID string
	Cmd    string
}

type SetPaneOptionCall struct {
	PaneID string
	Key    string
	Value  string
}

type SetSessionOptionCall struct {
	Session string
	Key     string
	Value   string
}

type SelectLayoutCall struct {
	Window string
	Layout string
}

type SetPaneTitleCall struct {
	PaneID string
	Title  string
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		NextPaneID:     1,
		Sessions:       map[string]bool{},
		PaneOptions:    map[string]map[string]string{},
		SessionOptions: map[string]map[string]string{},
		PaneCaptures:   map[string]string{},
		PanesBySession: map[string][]PaneInfo{},
	}
}

var _ Client = (*FakeClient)(nil)

func (f *FakeClient) NewSession(_ context.Context, spec SessionSpec) (string, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	f.NewSessionCalls = append(f.NewSessionCalls, spec)
	paneID := f.newPaneID()
	f.Sessions[spec.Name] = true
	f.PanesBySession[spec.Name] = append(f.PanesBySession[spec.Name], PaneInfo{
		PaneID:      paneID,
		SessionName: spec.Name,
		WindowName:  spec.WindowName,
		PaneIndex:   len(f.PanesBySession[spec.Name]),
		WindowIndex: 0,
		Active:      true,
	})

	return paneID, nil
}

func (f *FakeClient) SplitPane(_ context.Context, spec SplitSpec) (string, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	f.SplitPaneCalls = append(f.SplitPaneCalls, spec)
	paneID := f.newPaneID()
	session := f.findSessionByPane(spec.TargetPane)
	if session != "" {
		f.PanesBySession[session] = append(f.PanesBySession[session], PaneInfo{
			PaneID:      paneID,
			SessionName: session,
			PaneIndex:   len(f.PanesBySession[session]),
			WindowIndex: 0,
		})
	}

	return paneID, nil
}

func (f *FakeClient) SendKeys(_ context.Context, paneID string, text string, enter bool) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SendKeysCalls = append(f.SendKeysCalls, SendKeysCall{PaneID: paneID, Text: text, Enter: enter})
	if text != "" {
		f.PaneCaptures[paneID] += text
	}
	if enter {
		f.PaneCaptures[paneID] += "\n"
	}

	return nil
}

func (f *FakeClient) CapturePane(_ context.Context, paneID string, lines int) (string, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	f.CapturePaneCalls = append(f.CapturePaneCalls, CapturePaneCall{PaneID: paneID, Lines: lines})
	return f.PaneCaptures[paneID], nil
}

func (f *FakeClient) SetBuffer(_ context.Context, data string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SetBufferCalls = append(f.SetBufferCalls, data)
	f.Buffer = data
	return nil
}

func (f *FakeClient) PasteBuffer(_ context.Context, paneID string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.PasteBufferCalls = append(f.PasteBufferCalls, paneID)
	f.PaneCaptures[paneID] += f.Buffer
	return nil
}

func (f *FakeClient) PipePane(_ context.Context, paneID string, cmd string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.PipePaneCalls = append(f.PipePaneCalls, PipePaneCall{PaneID: paneID, Cmd: cmd})
	return nil
}

func (f *FakeClient) SetPaneOption(_ context.Context, paneID, key, value string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SetPaneOptionCalls = append(f.SetPaneOptionCalls, SetPaneOptionCall{PaneID: paneID, Key: key, Value: value})
	if f.PaneOptions[paneID] == nil {
		f.PaneOptions[paneID] = map[string]string{}
	}
	f.PaneOptions[paneID][key] = value
	return nil
}

func (f *FakeClient) ShowPaneOption(_ context.Context, paneID, key string) (string, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}

	if f.PaneOptions[paneID] == nil {
		return "", nil
	}
	return f.PaneOptions[paneID][key], nil
}

func (f *FakeClient) SetSessionOption(_ context.Context, session, key, value string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SetSessionOptionCalls = append(f.SetSessionOptionCalls, SetSessionOptionCall{Session: session, Key: key, Value: value})
	if f.SessionOptions[session] == nil {
		f.SessionOptions[session] = map[string]string{}
	}
	f.SessionOptions[session][key] = value
	return nil
}

func (f *FakeClient) ListPanes(_ context.Context, session string) ([]PaneInfo, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	panes := f.PanesBySession[session]
	out := make([]PaneInfo, len(panes))
	copy(out, panes)
	return out, nil
}

func (f *FakeClient) HasSession(_ context.Context, session string) (bool, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}
	return f.Sessions[session], nil
}

func (f *FakeClient) KillSession(_ context.Context, session string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.KillSessionCalls = append(f.KillSessionCalls, session)
	delete(f.Sessions, session)
	delete(f.PanesBySession, session)
	return nil
}

func (f *FakeClient) SelectLayout(_ context.Context, window, layout string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SelectLayoutCalls = append(f.SelectLayoutCalls, SelectLayoutCall{Window: window, Layout: layout})
	return nil
}

func (f *FakeClient) DisplayPopup(_ context.Context, spec PopupSpec) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.PopupErr != nil {
		return f.PopupErr
	}
	if f.Err != nil {
		return f.Err
	}

	f.DisplayPopupCalls = append(f.DisplayPopupCalls, spec)
	return nil
}

func (f *FakeClient) SetPaneTitle(_ context.Context, paneID, title string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.SetPaneTitleCalls = append(f.SetPaneTitleCalls, SetPaneTitleCall{PaneID: paneID, Title: title})
	for session, panes := range f.PanesBySession {
		for i := range panes {
			if panes[i].PaneID == paneID {
				panes[i].PaneTitle = title
				f.PanesBySession[session] = panes
				return nil
			}
		}
	}
	return nil
}

func (f *FakeClient) newPaneID() string {
	id := fmt.Sprintf("%%%d", f.NextPaneID)
	f.NextPaneID++
	return id
}

func (f *FakeClient) findSessionByPane(paneID string) string {
	for session, panes := range f.PanesBySession {
		for _, pane := range panes {
			if pane.PaneID == paneID {
				return session
			}
		}
	}
	return strings.TrimSpace("")
}
