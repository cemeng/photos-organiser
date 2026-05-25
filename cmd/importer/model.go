package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleTitle  = lipgloss.NewStyle().Bold(true)
	styleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleGood   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleError  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	stylePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

// ── Screen states ─────────────────────────────────────────────────────────────

type screen int

const (
	screenDestInput screen = iota
	screenScanning
	screenConfirm
	screenExecuting
	screenDone
)

// ── Tea messages ──────────────────────────────────────────────────────────────

type msgScanDone struct {
	plan *ImportPlan
	err  error
}

type msgFileResult struct {
	result FileResult
	index  int // index of the file just processed
}

type msgReportDone struct {
	err error
}

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	source string
	dest   string
	screen screen
	err    error

	input textinput.Model
	spin  spinner.Model
	prog  progress.Model

	plan    *ImportPlan
	report  *ImportReport
	execIdx int // next file index to process
}

func newModel(source string) model {
	ti := textinput.New()
	ti.Placeholder = DefaultDest(source)
	ti.SetValue(DefaultDest(source))
	ti.Focus()
	ti.Width = 60
	ti.Prompt = stylePrompt.Render("  Destination: ")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	pr := progress.New(progress.WithDefaultGradient())

	return model{
		source: source,
		input:  ti,
		spin:   sp,
		prog:   pr,
		screen: screenDestInput,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.screen != screenExecuting {
				return m, tea.Quit
			}
		default:
			if m.screen == screenDone {
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		pm, cmd := m.prog.Update(msg)
		m.prog = pm.(progress.Model)
		return m, cmd

	case msgScanDone:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenDone
			return m, nil
		}
		m.plan = msg.plan
		m.screen = screenConfirm
		return m, nil

	case msgFileResult:
		m.report.Results = append(m.report.Results, msg.result)
		m.execIdx = msg.index + 1
		total := len(m.plan.Files)
		pct := float64(m.execIdx) / float64(total)

		if m.execIdx >= total {
			// all files processed — write report
			return m, tea.Batch(
				m.prog.SetPercent(1.0),
				cmdFinalise(m.report),
			)
		}
		return m, tea.Batch(
			m.prog.SetPercent(pct),
			cmdProcessOne(m.plan, m.execIdx),
		)

	case msgReportDone:
		if msg.err != nil {
			m.err = msg.err
		}
		m.screen = screenDone
		return m, nil
	}

	switch m.screen {
	case screenDestInput:
		return m.updateDestInput(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	}

	return m, nil
}

func (m model) updateDestInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			dest := NormaliseDir(strings.TrimSpace(m.input.Value()))
			if dest == "" {
				dest = DefaultDest(m.source)
			}
			if err := ValidateDirectories(m.source, dest); err != nil {
				m.err = err
				m.screen = screenDone
				return m, nil
			}
			m.dest = dest
			m.screen = screenScanning
			return m, tea.Batch(m.spin.Tick, cmdScan(m.source, dest))
		}
	}
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			m.screen = screenExecuting
			m.execIdx = 0
			m.report = &ImportReport{
				StartedAt:   time.Now(),
				Source:      m.plan.Source,
				Destination: m.plan.Destination,
			}
			return m, tea.Batch(
				m.prog.SetPercent(0),
				cmdProcessOne(m.plan, 0),
			)
		case "n", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleTitle.Render("  Photos Importer"))
	b.WriteString("\n\n")
	b.WriteString(styleMuted.Render(fmt.Sprintf("  Source: %s", m.source)))
	b.WriteString("\n\n")

	switch m.screen {
	case screenDestInput:
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString(styleMuted.Render("  Press Enter to confirm, Ctrl+C to quit"))

	case screenScanning:
		b.WriteString(fmt.Sprintf("  %s Scanning source directory…", m.spin.View()))

	case screenConfirm:
		b.WriteString(viewPlan(m.plan))
		b.WriteString("\n")
		b.WriteString(stylePrompt.Render("  Proceed? [y/N]: "))

	case screenExecuting:
		total := len(m.plan.Files)
		b.WriteString(fmt.Sprintf("  Copying files… (%d / %d)\n\n", m.execIdx, total))
		b.WriteString("  " + m.prog.View())

	case screenDone:
		if m.err != nil {
			b.WriteString(styleError.Render(fmt.Sprintf("  Error: %v", m.err)))
		} else if m.report != nil {
			b.WriteString(viewReport(m.report))
		} else {
			b.WriteString(styleMuted.Render("  Aborted."))
		}
		b.WriteString("\n")
		b.WriteString(styleMuted.Render("  Press any key to exit."))
	}

	b.WriteString("\n\n")
	return b.String()
}

func viewPlan(p *ImportPlan) string {
	var b strings.Builder

	processable := 0
	for _, f := range p.Files {
		if f.Class == ClassProcessable {
			processable++
		}
	}
	skipped := len(p.Files) - processable

	b.WriteString(fmt.Sprintf("  Found %s processable files:\n",
		styleGood.Render(fmt.Sprintf("%d", processable))))

	dirs := make([]string, 0, len(p.Groups))
	for d := range p.Groups {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		b.WriteString(styleMuted.Render(fmt.Sprintf("    → %s%s  (%d files)\n",
			p.Destination, d, p.Groups[d])))
	}

	if skipped > 0 {
		b.WriteString(styleWarn.Render(fmt.Sprintf("\n  Skipped: %d file(s) (details in report)", skipped)))
		b.WriteString("\n")
	}

	return b.String()
}

func viewReport(r *ImportReport) string {
	var b strings.Builder

	b.WriteString(styleGood.Render("  Done!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Processed:  %s\n", styleGood.Render(fmt.Sprintf("%d", r.Processed()))))

	if len(r.Skipped()) > 0 {
		b.WriteString(fmt.Sprintf("  Skipped:    %s\n", styleWarn.Render(fmt.Sprintf("%d", len(r.Skipped())))))
	}
	if len(r.Collisions()) > 0 {
		b.WriteString(fmt.Sprintf("  Collisions: %s\n", styleWarn.Render(fmt.Sprintf("%d", len(r.Collisions())))))
	}
	if len(r.Errors()) > 0 {
		b.WriteString(fmt.Sprintf("  Errors:     %s\n", styleError.Render(fmt.Sprintf("%d", len(r.Errors())))))
	}

	b.WriteString(fmt.Sprintf("\n  Report written to:\n  %s\n",
		styleMuted.Render(r.ReportPath)))

	return b.String()
}

// ── Commands ──────────────────────────────────────────────────────────────────

func cmdScan(src, dest string) tea.Cmd {
	return func() tea.Msg {
		plan, err := ScanDir(src, dest)
		return msgScanDone{plan: plan, err: err}
	}
}

// cmdProcessOne processes a single file at index i and returns its result as a message.
func cmdProcessOne(plan *ImportPlan, i int) tea.Cmd {
	return func() tea.Msg {
		result := ExecuteOne(plan.Files[i], plan.Source)
		return msgFileResult{result: result, index: i}
	}
}

func cmdFinalise(report *ImportReport) tea.Cmd {
	return func() tea.Msg {
		err := FinaliseReport(report)
		return msgReportDone{err: err}
	}
}
