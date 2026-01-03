package ui

import (
	"fmt"
	"spotify-mini/internal/api"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	colActive = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#1DB954"}
	colText   = lipgloss.AdaptiveColor{Light: "#383838", Dark: "#D9DCCF"}
	colDim    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#5c5c5c"}

	windowStyle = lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colText).
			Bold(true).
			Align(lipgloss.Center)

	artistStyle = lipgloss.NewStyle().
			Foreground(colDim).
			Align(lipgloss.Center)

	btnStyle = lipgloss.NewStyle().
			Foreground(colDim).
			Padding(0, 1)

	btnActiveStyle = lipgloss.NewStyle().
			Foreground(colActive).
			Bold(true).
			Padding(0, 1)

	barContainerStyle = lipgloss.NewStyle().
				Foreground(colActive).
				Align(lipgloss.Center).
				MarginTop(0)

	queueTitleStyle = lipgloss.NewStyle().
			Foreground(colDim).
			Bold(true).
			MarginTop(1)

	queueItemStyle = lipgloss.NewStyle().
			Foreground(colDim)

	queueSelectedStyle = lipgloss.NewStyle().
				Foreground(colActive).
				Bold(true)
)

type Model struct {
	token *api.Token

	focusMode int

	choices   []string
	btnCursor int

	queue       []api.Item
	queueCursor int
	queueOffset int

	state  *api.PlayerState
	width  int
	height int
	msg    string
}

func InitialModel(t *api.Token) Model {
	return Model{
		token:     t,
		focusMode: 0,
		choices:   []string{"Skip Back", "Play/Pause", "Skip Forward"},
		btnCursor: 1,
		msg:       "",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchStatus(m.token), fetchQueue(m.token), tick())
}

func (m Model) queueViewHeight() int {
	const fixedOverhead = 9
	const maxRows = 10

	h := m.height
	if h == 0 {
		h = 24
	}

	available := h - fixedOverhead
	if available < 1 {
		return 0
	}
	if available > maxRows {
		return maxRows
	}
	return available
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case *api.PlayerState:
		m.state = msg
		return m, nil

	case []api.Item:
		m.queue = msg
		return m, nil

	case TickMsg:
		return m, tea.Batch(fetchStatus(m.token), fetchQueue(m.token), tick())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "up", "k":
			if m.focusMode == 1 {
				if m.queueCursor > 0 {
					m.queueCursor--
					if m.queueCursor < m.queueOffset {
						m.queueOffset--
					}
				} else {
					m.focusMode = 0
				}
			}

		case "down", "j":
			if m.focusMode == 0 {
				m.focusMode = 1
			} else {
				if m.queueCursor < len(m.queue)-1 {
					m.queueCursor++
					rows := m.queueViewHeight()
					if rows < 1 {
						rows = 1
					}
					if m.queueCursor >= m.queueOffset+rows {
						m.queueOffset++
					}
				}
			}

		case "left", "h":
			if m.focusMode == 0 && m.btnCursor > 0 {
				m.btnCursor--
			}

		case "right", "l":
			if m.focusMode == 0 && m.btnCursor < len(m.choices)-1 {
				m.btnCursor++
			}

		case "enter", " ":
			if m.focusMode == 0 {

				switch m.btnCursor {
				case 0:
					api.SendCommand("POST", "previous", m.token)
					m.msg = "Prev"
				case 1:
					if m.state != nil && m.state.IsPlaying {
						api.SendCommand("PUT", "pause", m.token)
						m.msg = "Paused"
					} else {
						api.SendCommand("PUT", "play", m.token)
						m.msg = "Playing"
					}
				case 2:
					api.SendCommand("POST", "next", m.token)
					m.msg = "Next"
				}
				time.Sleep(100 * time.Millisecond)
				return m, fetchStatus(m.token)
			} else if m.focusMode == 1 {

				if len(m.queue) > 0 && m.queueCursor < len(m.queue) {
					targetItem := m.queue[m.queueCursor]

					ctx := m.state.Context
					usedContext := false

					if ctx != nil && ctx.URI != "" && targetItem.URI != "" && (ctx.Type == "playlist" || ctx.Type == "album") {
						err := api.PlayContext(m.token, ctx.URI, targetItem.URI)
						if err == nil {
							m.msg = "Skipped to: " + targetItem.Name
							usedContext = true
						}
					}

					if !usedContext {
						var uris []string
						limit := 50
						count := 0
						for i := m.queueCursor; i < len(m.queue); i++ {
							if m.queue[i].URI != "" {
								uris = append(uris, m.queue[i].URI)
								count++
								if count >= limit {
									break
								}
							}
						}

						if len(uris) > 0 {
							api.PlaySong(m.token, uris)
							m.msg = "Playing: " + targetItem.Name
						}
					}

					time.Sleep(200 * time.Millisecond)
					return m, fetchStatus(m.token)
				}
			}
		}
	}
	return m, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func (m Model) View() string {
	w, h := m.width, m.height
	if w == 0 {
		w = 40
	}
	if h == 0 {
		h = 24
	}

	availWidth := w - 4
	if availWidth < 20 {
		availWidth = 30
	}

	song, artist := "Nothing Playing", "Spotify"
	progress := 0.0

	if m.state != nil && m.state.Item != nil {
		song = m.state.Item.Name
		artist = m.state.Item.Artists[0].Name
		if m.state.Item.DurationMs > 0 {
			progress = float64(m.state.ProgressMs) / float64(m.state.Item.DurationMs)
		}
	}

	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Width(availWidth).Render(truncate(song, availWidth-2)),
		artistStyle.Width(availWidth).Render(truncate(artist, availWidth-2)),
	)

	barWidth := 25
	filledLen := int(float64(barWidth) * progress)
	if filledLen > barWidth {
		filledLen = barWidth
	}

	filled := strings.Repeat("━", filledLen)
	empty := ""
	if barWidth > filledLen {
		empty = strings.Repeat("─", barWidth-filledLen)
	}

	progressBar := barContainerStyle.Width(availWidth).Render(
		lipgloss.NewStyle().Foreground(colActive).Render(filled) +
			"●" +
			lipgloss.NewStyle().Foreground(colDim).Render(empty),
	)

	buttons := []string{"⏮", "⏯", "⏭"}
	var renderedBtns []string
	for i, label := range buttons {
		st := btnStyle
		if m.focusMode == 0 && m.btnCursor == i {
			st = btnActiveStyle
			label = "[" + label + "]"
		} else if m.focusMode == 1 {
			st = st.Foreground(colDim)
		}
		renderedBtns = append(renderedBtns, st.Render(label))
	}
	controls := lipgloss.JoinHorizontal(lipgloss.Center, renderedBtns...)

	var queueView string
	itemsToShow := m.queueViewHeight()

	if len(m.queue) > 0 && itemsToShow > 0 {
		queueView += queueTitleStyle.Render("Next Up:") + "\n"

		end := m.queueOffset + itemsToShow
		if end > len(m.queue) {
			end = len(m.queue)
		}

		for i := m.queueOffset; i < end; i++ {
			item := m.queue[i]
			name := item.Name

			prefix := fmt.Sprintf("%d. ", i+1)
			prefixLen := len(prefix)

			maxNameLen := availWidth - prefixLen - 2

			if maxNameLen < 5 {
				maxNameLen = 5
			}

			name = truncate(name, maxNameLen)

			line := prefix + name

			st := queueItemStyle.Width(availWidth)
			if m.focusMode == 1 && m.queueCursor == i {
				st = queueSelectedStyle.Width(availWidth)
				line = "> " + line
			} else {
				line = "  " + line
			}
			queueView += st.Render(line) + "\n"
		}
		// if m.queueOffset > 0 {
		// 	queueView = "↑\n" + queueView
		// }
		// if end < len(m.queue) {
		// 	queueView += "↓"
		// }
	} else {
		queueView = ""
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		header,
		progressBar,
		controls,
		queueView,
	)
	return windowStyle.Width(w).Height(h).Render(content)
}

type TickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) })
}

func fetchStatus(t *api.Token) tea.Cmd {
	return func() tea.Msg {
		state, _ := api.GetCurrentPlaying(t)
		return state
	}
}

func fetchQueue(t *api.Token) tea.Cmd {
	return func() tea.Msg {
		q, _ := api.GetQueue(t)
		return q
	}
}
