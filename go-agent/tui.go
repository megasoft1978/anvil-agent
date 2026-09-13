package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// traceEvent is the one typed message every trace.Event() call produces, delivered to the TUI
// through the same Trace.Sink hook the headless JSONL logger uses — no separate event system,
// no prompt or runAgent duplication.
type traceEvent struct {
	kind string
	data any
	at   time.Time
}

type agentDoneMsg struct{ result Summary }

// tuiModel is a thin view over runAgent: it never constructs prompts or reinterprets tool
// results, it only renders the events runAgent already emits.
type tuiModel struct {
	prompt     string
	model      string
	repoRoot   string
	transcript []string
	maxLines   int
	status     string // "waiting for model" | "running: <tool>" | "done" | "error"
	turns      int
	toolCalls  int
	edits      int
	started    time.Time
	finished   bool
	result     Summary
	width      int
	height     int
	cancel     context.CancelFunc
}

const tuiMaxTranscriptLines = 2000

func newTUIModel(prompt, model, repoRoot string, cancel context.CancelFunc) tuiModel {
	return tuiModel{
		prompt:   prompt,
		model:    model,
		repoRoot: repoRoot,
		maxLines: tuiMaxTranscriptLines,
		status:   "waiting for model",
		started:  time.Now(),
		cancel:   cancel,
	}
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m *tuiModel) append(line string) {
	for _, part := range strings.Split(line, "\n") {
		m.transcript = append(m.transcript, part)
	}
	if len(m.transcript) > m.maxLines {
		m.transcript = m.transcript[len(m.transcript)-m.maxLines:]
	}
}

// summarize renders one trace event as a single compact transcript line. It never invents
// percentages, diagnoses, or content the event does not carry.
func summarize(ev traceEvent) (line string, status string) {
	stamp := ev.at.Format("15:04:05")
	switch ev.kind {
	case "request":
		return fmt.Sprintf("[%s] waiting for model...", stamp), "waiting for model"
	case "tool_start":
		call, _ := ev.data.(*ToolCall)
		name := "?"
		if call != nil {
			name = call.Function.Name
		}
		return fmt.Sprintf("[%s] -> %s", stamp, name), "running: " + name
	case "tool_result":
		fields, _ := ev.data.(map[string]any)
		name, _ := fields["name"].(string)
		output, _ := fields["output"].(map[string]any)
		if errText, ok := output["error"].(string); ok {
			return fmt.Sprintf("[%s] <- %s failed: %s", stamp, name, errText), "waiting for model"
		}
		return fmt.Sprintf("[%s] <- %s ok", stamp, name), "waiting for model"
	case "recovery":
		return fmt.Sprintf("[%s] recovered a malformed tool call", stamp), ""
	case "correction":
		reason, _ := ev.data.(map[string]any)["reason"].(string)
		return fmt.Sprintf("[%s] correction sent to model: %s", stamp, reason), ""
	case "summary":
		return fmt.Sprintf("[%s] finished", stamp), "done"
	}
	return "", ""
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case traceEvent:
		switch msg.kind {
		case "tool_start":
			m.toolCalls++
		}
		if msg.kind == "tool_result" {
			if fields, ok := msg.data.(map[string]any); ok {
				if output, ok := fields["output"].(map[string]any); ok {
					if result, ok := output["result"].(map[string]any); ok {
						if applied, _ := result["applied"].(bool); applied {
							m.edits++
						}
					}
				}
			}
		}
		if msg.kind == "request" {
			m.turns++
		}
		line, status := summarize(msg)
		if line != "" {
			m.append(line)
		}
		if status != "" {
			m.status = status
		}
		return m, nil
	case agentDoneMsg:
		m.finished = true
		m.result = msg.result
		m.status = "done: " + msg.result.Status
		if msg.result.Answer != "" {
			m.append("")
			m.append("answer: " + msg.result.Answer)
		}
		if msg.result.Error != "" {
			m.append("error: " + msg.result.Error)
		}
		return m, nil
	}
	return m, nil
}

func (m tuiModel) View() string {
	elapsed := time.Since(m.started).Round(time.Second)
	header := fmt.Sprintf("anvil-agent  model=%s  repo=%s  elapsed=%s  turns=%d  tools=%d  edits=%d\n",
		m.model, m.repoRoot, elapsed, m.turns, m.toolCalls, m.edits)
	status := "status: " + m.status + "\n"
	body := strings.Join(m.transcript, "\n")
	help := "\n(ctrl+c or q to cancel and exit)"
	visible := m.height - 5
	lines := strings.Split(body, "\n")
	if visible > 0 && len(lines) > visible {
		lines = lines[len(lines)-visible:]
	}
	return header + status + strings.Join(lines, "\n") + help
}

// runTUI drives one runAgent call through a Bubble Tea program. It sets Trace.Sink to forward
// every event as a tea.Msg; the JSONL trace file (if configured) keeps receiving the same
// events unchanged, so headless and TUI runs produce identical artifacts.
func runTUI(ctx context.Context, config Config, prompt string, client *Client, tools *Tools, trace *Trace, opts ...tea.ProgramOption) (Summary, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	model := newTUIModel(prompt, config.Model, tools.Root.Name(), cancel)
	program := tea.NewProgram(model, opts...)
	trace.Sink = func(kind string, data any, at time.Time) {
		program.Send(traceEvent{kind: kind, data: data, at: at})
	}
	resultCh := make(chan Summary, 1)
	go func() {
		resultCh <- runAgent(ctx, config, prompt, client, tools, trace)
	}()
	go func() {
		result := <-resultCh
		program.Send(agentDoneMsg{result: result})
	}()
	finalModel, err := program.Run()
	if err != nil {
		return Summary{}, err
	}
	final, _ := finalModel.(tuiModel)
	if final.finished {
		return final.result, nil
	}
	// The user quit before the agent finished; the goroutine above is still running against
	// the cancelled context and will return promptly once its next ctx.Err() check fires.
	return <-resultCh, nil
}
