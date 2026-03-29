package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coyaSONG/tmuxicate/internal/config"
	"github.com/fsnotify/fsnotify"
)

type LogOpts struct {
	Tail       int
	Follow     bool
	All        bool
	Raw        bool
	EventsOnly bool
}

type logEntry struct {
	Timestamp time.Time
	Agent     string
	Line      string
}

type eventLine struct {
	Timestamp string `json:"ts"`
	Event     string `json:"event"`
	Schema    string `json:"schema"`
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func LogView(stateDir string, agent string, opts LogOpts) error {
	cfg, err := loadResolvedConfigFromStateDir(stateDir)
	if err != nil {
		return err
	}
	if opts.Tail <= 0 {
		opts.Tail = 100
	}

	agents, err := logAgents(cfg, agent, opts.All)
	if err != nil {
		return err
	}

	entries, files, err := collectLogEntries(stateDir, agents, opts)
	if err != nil {
		return err
	}

	if len(entries) > opts.Tail {
		entries = entries[len(entries)-opts.Tail:]
	}
	for _, entry := range entries {
		fmt.Println(formatLogEntry(entry, opts.All))
	}

	if !opts.Follow {
		return nil
	}

	return followLogs(files, opts)
}

func collectLogEntries(stateDir string, agents []string, opts LogOpts) ([]logEntry, []watchedFile, error) {
	var entries []logEntry
	var watched []watchedFile

	for _, agent := range agents {
		if !opts.EventsOnly {
			transcriptPath := transcriptPath(stateDir, agent, opts.Raw)
			transcriptEntries, err := loadTranscriptEntries(agent, transcriptPath, opts.Raw)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, transcriptEntries...)
			watched = append(watched, watchedFile{Path: transcriptPath, Agent: agent, Kind: "transcript", Raw: opts.Raw})
		}

		eventPaths := []string{
			filepath.Join(stateDir, "agents", agent, "events", "state.jsonl"),
			filepath.Join(stateDir, "agents", agent, "events", "observed.current.json"),
		}
		for _, eventPath := range eventPaths {
			eventEntries, err := loadEventEntries(agent, eventPath)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, eventEntries...)
			watched = append(watched, watchedFile{Path: eventPath, Agent: agent, Kind: "event"})
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Timestamp.Equal(entries[j].Timestamp) {
			if entries[i].Agent == entries[j].Agent {
				return entries[i].Line < entries[j].Line
			}
			return entries[i].Agent < entries[j].Agent
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries, watched, nil
}

type watchedFile struct {
	Path  string
	Agent string
	Kind  string
	Raw   bool
}

func followLogs(files []watchedFile, opts LogOpts) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	offsets := map[string]int64{}
	seen := map[string]watchedFile{}
	watchedDirs := map[string]struct{}{}

	for _, file := range files {
		seen[file.Path] = file
		dir := filepath.Dir(file.Path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if _, ok := watchedDirs[dir]; !ok {
			if err := watcher.Add(dir); err != nil {
				return err
			}
			watchedDirs[dir] = struct{}{}
		}
		offsets[file.Path] = fileSize(file.Path)
	}

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			file, ok := seen[ev.Name]
			if !ok {
				continue
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}
			lines, nextOffset, err := readNewLines(file, offsets[file.Path])
			if err != nil {
				return err
			}
			offsets[file.Path] = nextOffset
			for _, line := range lines {
				fmt.Println(formatLogEntry(line, opts.All))
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

func readNewLines(file watchedFile, offset int64) ([]logEntry, int64, error) {
	f, err := os.Open(file.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, offset, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset, err
	}
	nextOffset := offset + int64(len(data))
	if len(data) == 0 {
		return nil, nextOffset, nil
	}

	now := time.Now().UTC()
	var entries []logEntry
	switch file.Kind {
	case "event":
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			entry := parseEventLogLine(file.Agent, line)
			if entry.Timestamp.IsZero() {
				entry.Timestamp = now
			}
			entries = append(entries, entry)
		}
	default:
		content := string(data)
		if !file.Raw {
			content = stripANSI(content)
		}
		for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			entries = append(entries, logEntry{
				Timestamp: now,
				Agent:     file.Agent,
				Line:      line,
			})
		}
	}

	return entries, nextOffset, nil
}

func loadTranscriptEntries(agent, path string, raw bool) ([]logEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if !raw {
		content = stripANSI(content)
	}

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	entries := make([]logEntry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entries = append(entries, logEntry{
			Timestamp: info.ModTime().UTC(),
			Agent:     agent,
			Line:      line,
		})
	}
	return entries, nil
}

func loadEventEntries(agent, path string) ([]logEntry, error) {
	if filepath.Ext(path) == ".json" {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		line := strings.TrimSpace(string(data))
		if line == "" {
			return nil, nil
		}
		return []logEntry{parseEventLogLine(agent, line)}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var entries []logEntry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entries = append(entries, parseEventLogLine(agent, line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func parseEventLogLine(agent, line string) logEntry {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return logEntry{
			Timestamp: time.Now().UTC(),
			Agent:     agent,
			Line:      line,
		}
	}

	ts := time.Now().UTC()
	if rawTS, ok := payload["ts"].(string); ok && strings.TrimSpace(rawTS) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, rawTS); err == nil {
			ts = parsed
		}
	}

	text := renderEventPayload(payload)
	return logEntry{
		Timestamp: ts,
		Agent:     agent,
		Line:      text,
	}
}

func renderEventPayload(payload map[string]any) string {
	if event, ok := payload["event"].(string); ok && event != "" {
		parts := []string{fmt.Sprintf("[event] %s", event)}
		if state, ok := payload["declared_state"].(string); ok && state != "" {
			parts = append(parts, fmt.Sprintf("declared=%s", state))
		}
		if state, ok := payload["observed_state"].(string); ok && state != "" {
			parts = append(parts, fmt.Sprintf("observed=%s", state))
		}
		if msgID, ok := payload["message_id"].(string); ok && msgID != "" {
			parts = append(parts, fmt.Sprintf("message=%s", msgID))
		}
		if reason, ok := payload["reason"].(string); ok && reason != "" {
			parts = append(parts, fmt.Sprintf("reason=%s", reason))
		}
		if summary, ok := payload["summary"].(string); ok && summary != "" {
			parts = append(parts, fmt.Sprintf("summary=%s", summary))
		}
		return strings.Join(parts, " ")
	}
	if state, ok := payload["observed_state"].(string); ok && state != "" {
		return fmt.Sprintf("[state] observed=%s", state)
	}
	if data, err := json.Marshal(payload); err == nil {
		return string(data)
	}
	return fmt.Sprintf("%v", payload)
}

func formatLogEntry(entry logEntry, includeAgent bool) string {
	prefix := entry.Timestamp.Format(time.RFC3339)
	if includeAgent {
		return fmt.Sprintf("%s [%s] %s", prefix, entry.Agent, entry.Line)
	}
	return fmt.Sprintf("%s %s", prefix, entry.Line)
}

func logAgents(cfg *config.ResolvedConfig, requested string, all bool) ([]string, error) {
	if all {
		agents := make([]string, 0, len(cfg.Agents))
		for _, agent := range cfg.Agents {
			agents = append(agents, agent.Name)
		}
		return agents, nil
	}
	if strings.TrimSpace(requested) == "" {
		return nil, fmt.Errorf("agent is required unless --all is set")
	}
	agentName, err := resolveTargetAgent(cfg, requested)
	if err != nil {
		return nil, err
	}
	return []string{agentName}, nil
}

func transcriptPath(stateDir, agent string, raw bool) string {
	if !raw {
		plain := filepath.Join(stateDir, "agents", agent, "transcripts", "plain.txt")
		if _, err := os.Stat(plain); err == nil {
			return plain
		}
	}
	return filepath.Join(stateDir, "agents", agent, "transcripts", "raw.ansi.log")
}

func stripANSI(value string) string {
	return ansiRegexp.ReplaceAllString(value, "")
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
