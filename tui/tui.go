package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ozorg.xyz/flapjack/build"
	"ozorg.xyz/flapjack/cache"
)

var (
	red     = lipgloss.Color("196")
	darkRed = lipgloss.Color("124")
	orange  = lipgloss.Color("202")
	gray    = lipgloss.Color("243")
	white   = lipgloss.Color("255")
	green   = lipgloss.Color("118")
	dgray   = lipgloss.Color("245")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(red)
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(orange)
	responseStyle = lipgloss.NewStyle().Foreground(white)
	successStyle = lipgloss.NewStyle().Foreground(green)
	errorStyle = lipgloss.NewStyle().Bold(true).Foreground(red)
	infoStyle = lipgloss.NewStyle().Foreground(dgray)
	separatorStyle = lipgloss.NewStyle().Foreground(darkRed)
	systemStyle = lipgloss.NewStyle().Foreground(gray)
	borderStyle = lipgloss.NewStyle().Foreground(darkRed)
	inputPromptStyle = lipgloss.NewStyle().Bold(true).Foreground(red)
	suggestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
)

type (
	appendMsg string
	exitMsg   struct{}
)

type Model struct {
	ready       bool
	viewport    viewport.Model
	input       textinput.Model
	builder     *build.Build
	store       *cache.Store
	convStore   *cache.ConvStore
	content     []string
	suggestions  []string
	showSuggest  bool
	width       int
	height      int
	convID      string
	convName    string
	convDirty   bool
}

func New(b *build.Build, s *cache.Store) *Model {
	ti := textinput.New()
	ti.Placeholder = "digite uma mensagem ou :help"
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 80

	m := &Model{
		input:     ti,
		builder:   b,
		store:     s,
		convStore: cache.NewConvStore(s),
	}

	m.content = append(m.content, m.renderHeader())
	m.content = append(m.content, checkSetup(s, b))
	m.content = append(m.content, "")
	return m
}

func checkSetup(s *cache.Store, b *build.Build) string {
	_, groqErr := s.Get("GROQ_API_KEY")
	_, pteroURLErr := s.Get("PTERODACTYL_URL")
	_, pteroKeyErr := s.Get("PTERODACTYL_API_KEY")

	hasGroq := groqErr == nil
	hasPtero := pteroURLErr == nil && pteroKeyErr == nil

	var lines []string
	lines = append(lines, separatorStyle.Render("  ────────────────  SETUP  ────────────────"))

	if !hasGroq {
		lines = append(lines, errorStyle.Render("  ✘ GROQ_API_KEY nao configurada"))
		lines = append(lines, infoStyle.Render("     :config set GROQ_API_KEY <sua-chave>"))
		lines = append(lines, infoStyle.Render("     O app NAO funcionara sem a chave da IA."))
		lines = append(lines, "")
	}

	if !hasPtero {
		lines = append(lines, infoStyle.Render("  ! Pterodactyl nao configurado"))
		lines = append(lines, infoStyle.Render("     :config set PTERODACTYL_URL <url>"))
		lines = append(lines, infoStyle.Render("     :config set PTERODACTYL_API_KEY <key>"))
		lines = append(lines, infoStyle.Render("     Sem isso, o app funciona apenas como chat."))
	} else {
		lines = append(lines, successStyle.Render("  ✓ Pterodactyl configurado"))
	}

	lines = append(lines, separatorStyle.Render("  ─────────────────────────────────────────"))

	return strings.Join(lines, "\n")
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-8)
			m.viewport.Style = lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2)
			m.input.Width = msg.Width - 8
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - 8
			m.input.Width = msg.Width - 8
		}
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit

		case "enter":
			val := m.input.Value()
			m.input.SetValue("")
			m.showSuggest = false

			if val == "" {
				return m, nil
			}

			if val == "exit" {
				m.content = append(m.content, "", m.renderGoodbye())
				m.refreshViewport()
				return m, tea.Quit
			}

			return m, m.handleInput(val)

		case "tab":
			if len(m.suggestions) == 1 {
				m.input.SetValue(m.suggestions[0])
				m.input.SetCursor(len(m.suggestions[0]))
				m.showSuggest = false
				m.suggestions = nil
			}
			return m, nil

		case "esc":
			m.showSuggest = false
			m.suggestions = nil

		default:
			if m.ready {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
				m.updateSuggestions()
			}
			return m, tea.Batch(cmds...)
		}

	case appendMsg:
		m.content = append(m.content, string(msg))
		m.refreshViewport()
		return m, nil

	case exitMsg:
		return m, tea.Quit
	}

	if m.ready {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if !m.ready {
		return titleStyle.Render("carregando...")
	}

	promptLabel := "❯ "
	if m.convID != "" {
		promptLabel = fmt.Sprintf("conv:%.6s ❯ ", m.convID)
	}

	inputLine := inputPromptStyle.Render(promptLabel) + m.input.View()

	suggestBox := ""
	if m.showSuggest && len(m.suggestions) > 0 {
		var lines []string
		for _, s := range m.suggestions {
			lines = append(lines, suggestionStyle.Render("  "+s))
		}
		suggestBox = "\n" + strings.Join(lines, "\n")
	}

	return m.viewport.View() + "\n" + inputLine + suggestBox
}

func (m *Model) updateSuggestions() {
	val := m.input.Value()
	m.suggestions = nil
	m.showSuggest = false

	if val == "" {
		return
	}

	suggestions := getSuggestions(val)
	if len(suggestions) > 0 {
		m.suggestions = suggestions
		m.showSuggest = true
	}
}

func getSuggestions(input string) []string {
	var matches []string
	all := []string{
		":help",
		":v",
		":config set",
		":config get",
		":config list",
		":config delete",
		":conv save",
		":conv list",
		":conv load",
		":conv new",
		":conv delete",
		"exit",
	}

	lower := strings.ToLower(input)
	for _, cmd := range all {
		if strings.HasPrefix(strings.ToLower(cmd), lower) && cmd != input {
			matches = append(matches, cmd)
		}
	}

	if len(matches) > 5 {
		matches = matches[:5]
	}

	return matches
}

func (m *Model) handleInput(val string) tea.Cmd {
	m.content = append(m.content, inputPromptStyle.Render("❯ ")+responseStyle.Render(val))

	switch {
	case val == ":help":
		m.content = append(m.content, m.renderHelp()...)

	case val == ":v":
		v := !m.builder.IsVerbose()
		m.builder.SetVerbose(v)
		if v {
			m.content = append(m.content, accentStyle.Render("  ▸ verbose ON"))
		} else {
			m.content = append(m.content, systemStyle.Render("  ▸ verbose OFF"))
		}

	case strings.HasPrefix(val, ":config"):
		lines := handleConfig(m.store, val)
		m.content = append(m.content, lines...)

	case strings.HasPrefix(val, ":conv"):
		lines := m.handleConv(val)
		m.content = append(m.content, lines...)

	default:
		m.content = append(m.content,
			separatorStyle.Render("  ─────────────────"),
			accentStyle.Render("  ⚡ processando..."),
		)
		m.refreshViewport()

		return func() tea.Msg {
			resp, err := m.builder.Generate(context.Background(), val)
			if err != nil {
				if len(m.content) > 0 {
					m.content = m.content[:len(m.content)-1]
				}
				return appendMsg(errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err)))
			}
			return appendMsg(responseStyle.Render(resp) + "\n")
		}
	}

	m.refreshViewport()
	return nil
}

func (m *Model) refreshViewport() {
	m.viewport.SetContent(strings.Join(m.content, "\n"))
	m.viewport.GotoBottom()
}

func (m *Model) renderHeader() string {
	pteroStatus := infoStyle.Render("  Pterodactyl: chat")
	if m.builder.Ptero != nil {
		pteroStatus = successStyle.Render("  Pterodactyl: conectado")
	}

	lines := []string{
		titleStyle.Render(`
███████╗██╗      █████╗ ██████╗     ██╗ █████╗  ██████╗██╗  ██╗
██╔════╝██║     ██╔══██╗██╔══██╗    ██║██╔══██╗██╔════╝██║ ██╔╝
█████╗  ██║     ███████║██████╔╝    ██║███████║██║     █████╔╝
██╔══╝  ██║     ██╔══██║██╔═══╝ ██╗ ██║██╔══██║██║     ██╔═██╗
██║     ███████╗██║  ██║██║     ╚████╔╝██║  ██║╚██████╗██║  ██╗
╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝      ╚═══╝ ╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝
`),
		accentStyle.Render("  █████  PTERODACTYL  OPERATIONAL  CLI  █████"),
		pteroStatus,
		borderStyle.Render("  ─────────────────────────────────────────────"),
		infoStyle.Render("  :help        mostra os comandos disponiveis"),
		infoStyle.Render("  :config      gerencia chaves criptografadas"),
		infoStyle.Render("  :v           alterna modo verbose"),
		borderStyle.Render("  ─────────────────────────────────────────────"),
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderHelp() []string {
	return []string{
		accentStyle.Render("  ▸ COMANDOS DISPONIVEIS"),
		separatorStyle.Render("  ─────────────────────"),
		infoStyle.Render("  <texto>            enviar mensagem para IA"),
		infoStyle.Render("  :config set        salvar chave (criptografada)"),
		infoStyle.Render("  :config get        exibir chave"),
		infoStyle.Render("  :config list       listar chaves salvas"),
		infoStyle.Render("  :config delete     remover chave"),
		infoStyle.Render("  :conv save <nome>  salvar conversa atual"),
		infoStyle.Render("  :conv list         listar conversas salvas"),
		infoStyle.Render("  :conv load <id>    restaurar conversa"),
		infoStyle.Render("  :conv new          nova conversa"),
		infoStyle.Render("  :conv delete <id>  remover conversa"),
		infoStyle.Render("  :v                 alternar verbose"),
		infoStyle.Render("  :help              mostrar esta ajuda"),
		infoStyle.Render("  exit               sair"),
	}
}

func (m *Model) renderGoodbye() string {
	return strings.Join([]string{
		separatorStyle.Render("  ─────────────────────────────────────────────"),
		titleStyle.Render("  ███████╗██╗      █████╗ ██████╗     ██╗ █████╗  ██████╗██╗  ██╗"),
		titleStyle.Render("  ██╔════╝██║     ██╔══██╗██╔══██╗    ██║██╔══██╗██╔════╝██║ ██╔╝"),
		titleStyle.Render("  █████╗  ██║     ███████║██████╔╝    ██║███████║██║     █████╔╝ "),
		titleStyle.Render("  ██╔══╝  ██║     ██╔══██║██╔═══╝ ██╗ ██║██╔══██║██║     ██╔═██╗ "),
		titleStyle.Render("  ██║     ███████╗██║  ██║██║     ╚████╔╝██║  ██║╚██████╗██║  ██╗"),
		titleStyle.Render("  ╚═╝     ╚══════╝╚═╝  ╚═╝╚═╝      ╚═══╝ ╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝"),
	}, "\n")
}

func (m *Model) handleConv(val string) []string {
	parts := strings.Fields(val)
	if len(parts) < 2 {
		return []string{infoStyle.Render("  uso: :conv <save|load|list|new|delete> [nome|id]")}
	}

	cmd := parts[1]

	switch cmd {
	case "save":
		name := strings.TrimSpace(strings.TrimPrefix(val, ":conv save"))
		if name == "" {
			name = fmt.Sprintf("Conversa %s", time.Now().Format("02/01 15:04"))
		}

		msgs := m.builder.IA.ExportMessages()
		if len(msgs) <= 1 {
			return []string{infoStyle.Render("  conversa vazia, nada pra salvar")}
		}

		id, err := m.convStore.Save(name, msgs)
		if err != nil {
			return []string{errorStyle.Render(fmt.Sprintf("  ✘ erro ao salvar: %v", err))}
		}

		m.convID = id
		m.convName = name
		m.convDirty = false

		return []string{
			successStyle.Render(fmt.Sprintf("  ✓ conversa salva: %s", name)),
			infoStyle.Render(fmt.Sprintf("    id: %s", id)),
		}

	case "load":
		if len(parts) < 3 {
			return []string{infoStyle.Render("  uso: :conv load <id>")}
		}

		id := parts[2]
		msgs, err := m.convStore.Load(id)
		if err != nil {
			return []string{errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err))}
		}

		m.builder.IA.ImportMessages(msgs)
		m.convID = id
		m.convDirty = false

		return []string{
			successStyle.Render(fmt.Sprintf("  ✓ conversa restaurada (%d mensagens)", len(msgs))),
		}

	case "list":
		convs, err := m.convStore.List()
		if err != nil {
			return []string{errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err))}
		}

		if len(convs) == 0 {
			return []string{infoStyle.Render("  nenhuma conversa salva.")}
		}

		var lines []string
		lines = append(lines, accentStyle.Render("  ▸ CONVERSAS SALVAS"))
		lines = append(lines, separatorStyle.Render("  ─────────────────"))

		for _, c := range convs {
			mark := ""
			if c.ID == m.convID {
				mark = accentStyle.Render(" ◀")
			}
			lines = append(lines, infoStyle.Render(
				fmt.Sprintf("  %s  %s  [%d msgs]%s", c.ID[:8], c.Name, c.MessageCount, mark)))
		}

		return lines

	case "new":
		m.builder.IA.ClearContext()
		m.convID = ""
		m.convName = ""
		m.convDirty = false
		return []string{accentStyle.Render("  ▸ nova conversa iniciada")}

	case "delete":
		if len(parts) < 3 {
			return []string{infoStyle.Render("  uso: :conv delete <id>")}
		}

		id := parts[2]
		if err := m.convStore.Delete(id); err != nil {
			return []string{errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err))}
		}

		if id == m.convID {
			m.convID = ""
			m.convName = ""
		}

		return []string{successStyle.Render(fmt.Sprintf("  ✓ conversa %s removida", id[:8]))}

	default:
		return []string{infoStyle.Render("  comando desconhecido. use: save, load, list, new, delete")}
	}
}

func handleConfig(store *cache.Store, input string) []string {
	var lines []string
	lines = append(lines, accentStyle.Render("  ▸ CONFIG"))
	lines = append(lines, separatorStyle.Render("  ─────────────"))

	parts := strings.Fields(input)
	if len(parts) < 2 {
		lines = append(lines, infoStyle.Render("  uso: :config <set|get|list|delete> [chave] [valor]"))
		return lines
	}

	cmd := parts[1]
	switch cmd {
	case "set":
		if len(parts) < 4 {
			lines = append(lines, infoStyle.Render("  uso: :config set <CHAVE> <valor>"))
			return lines
		}
		key := parts[2]
		value := strings.Join(parts[3:], " ")
		if err := store.Set(key, value); err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err)))
			return lines
		}
		lines = append(lines, successStyle.Render(fmt.Sprintf("  ✓ %s salvo com seguranca", key)))

	case "get":
		if len(parts) < 3 {
			lines = append(lines, infoStyle.Render("  uso: :config get <CHAVE>"))
			return lines
		}
		val, err := store.Get(parts[2])
		if err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err)))
			return lines
		}
		lines = append(lines, successStyle.Render(fmt.Sprintf("  %s = %s", parts[2], val)))

	case "list":
		keys, err := store.List()
		if err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err)))
			return lines
		}
		if len(keys) == 0 {
			lines = append(lines, infoStyle.Render("  nenhuma chave configurada."))
			return lines
		}
		lines = append(lines, infoStyle.Render("  chaves:"))
		for k := range keys {
			lines = append(lines, infoStyle.Render(fmt.Sprintf("    • %s", k)))
		}

	case "delete":
		if len(parts) < 3 {
			lines = append(lines, infoStyle.Render("  uso: :config delete <CHAVE>"))
			return lines
		}
		if err := store.Delete(parts[2]); err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  ✘ erro: %v", err)))
			return lines
		}
		lines = append(lines, successStyle.Render(fmt.Sprintf("  ✓ %s removido.", parts[2])))

	default:
		lines = append(lines, infoStyle.Render("  comando desconhecido. use: set, get, list, delete"))
	}

	return lines
}
