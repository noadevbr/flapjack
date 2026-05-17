package build

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	flaphttp "ozorg.xyz/flapjack/http"
	"ozorg.xyz/flapjack/ia"
)

type Build struct {
	IA             *ia.IA
	Ptero          *flaphttp.Pterodactyl
	port           int
	currentContent string
	contentMu      sync.RWMutex
	verbose        bool
	verboseMu      sync.RWMutex
}

func (b *Build) SetVerbose(v bool) {
	b.verboseMu.Lock()
	defer b.verboseMu.Unlock()
	b.verbose = v
}

func (b *Build) IsVerbose() bool {
	b.verboseMu.RLock()
	defer b.verboseMu.RUnlock()
	return b.verbose
}

func NewBuild(ai *ia.IA, ptero *flaphttp.Pterodactyl) *Build {
	return &Build{
		IA:    ai,
		Ptero: ptero,
		port:  3248,
	}
}

func (b *Build) Generate(ctx context.Context, message string) (string, error) {

	resp, err := b.IA.Generate(ctx, message)
	if err != nil {
		return "", err
	}

	resp, err = b.HandleActions(ctx, resp)
	if err != nil {
		return "", err
	}

	if b.HasVisual(resp) {
		b.SetContent(resp)
		fmt.Printf("\n📊 Dashboard: http://localhost:%d\n", b.port)
	}

	if !b.IsVerbose() {
		resp = stripActionTags(resp)
	}

	resp = b.ConvertTerminalButtons(resp)

	return resp, nil
}

func (b *Build) HasVisual(content string) bool {
	return strings.Contains(content, "<re ")
}

func (b *Build) ExtractBlock(content string) string {
	start := strings.Index(content, "<re")
	if start == -1 {
		return content
	}

	end := strings.Index(content, "</re>")
	if end == -1 {
		return content
	}

	return content[start : end+5]
}

func (b *Build) HandleActions(ctx context.Context, content string) (string, error) {
	if b.Ptero == nil {
		return stripActionTags(content), nil
	}

	re := regexp.MustCompile(`<getInfo\s+([^>]+)\s*/>`)
	matches := re.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return content, nil
	}

	for _, m := range matches {
		fullTag := m[0]
		attrs := m[1]

		actionId := extractAttr(attrs, "actionId")
		if actionId == "" {
			continue
		}

		switch actionId {
		case "listServersAndPower":
			return b.handleListServers(ctx, content, fullTag)

		case "serverConsole":
			return b.handleServerConsole(ctx, content, fullTag)
		}
	}

	return content, nil
}

func extractAttr(attrs, name string) string {
	re := regexp.MustCompile(name + `="([^"]*)"`)
	m := re.FindStringSubmatch(attrs)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func (b *Build) handleListServers(ctx context.Context, content, fullTag string) (string, error) {

	servers, err := b.Ptero.GetServers()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("SERVERS CONTEXT:\n")

	for _, s := range servers.Data {

		d, err := b.Ptero.GetServer(s.Attributes.Identifier)
		if err != nil {
			continue
		}

		// STATUS só vem da chamada individual (GetServer), NÃO da listagem (GetServers)
		sb.WriteString(fmt.Sprintf(
			"NAME=%s ID=%s STATUS=%s CPU=%d RAM=%d DISK=%d NODE=%s\n",
			d.Attributes.Name,
			d.Attributes.Identifier,
			d.Attributes.Status,
			d.Attributes.Limits.CPU,
			d.Attributes.Limits.Memory,
			d.Attributes.Limits.Disk,
			d.Attributes.Node,
		))
	}

	b.IA.AddSystemContext(sb.String())

	clean := strings.ReplaceAll(content, fullTag, "")

	return b.IA.Generate(ctx, clean)
}

func (b *Build) handleServerConsole(ctx context.Context, content, fullTag string) (string, error) {
	serverId := extractAttr(fullTag, "serverId")
	if serverId == "" {
		return content, nil
	}

	server, err := b.Ptero.GetServer(serverId)
	if err != nil {
		return "", err
	}

	durationStr := extractAttr(fullTag, "duration")
	duration := 15 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err == nil && d > 0 {
			duration = time.Duration(d) * time.Second
		}
	}

	logs, err := b.Ptero.GetConsole(serverId, duration)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"SERVER STATUS: %s | SERVER NAME: %s | SERVER ID: %s\n",
		server.Attributes.Status,
		server.Attributes.Name,
		server.Attributes.Identifier,
	))
	sb.WriteString(fmt.Sprintf(
		"CONSOLE LOGS for server %s (last %v):\n", serverId, duration,
	))
	for _, line := range logs {
		sb.WriteString(line + "\n")
	}
	if len(logs) == 0 {
		sb.WriteString("(no console output in this period — server may be idle)\n")
	}

	b.IA.AddSystemContext(sb.String())

	clean := strings.ReplaceAll(content, fullTag, "")

	return b.IA.Generate(ctx, clean)
}

func (b *Build) SetContent(content string) {
	b.contentMu.Lock()
	defer b.contentMu.Unlock()
	b.currentContent = content
}

func (b *Build) GetContent() string {
	b.contentMu.RLock()
	defer b.contentMu.RUnlock()
	return b.currentContent
}

func (b *Build) StartServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, renderHTML(b.ConvertButtons(b.GetContent())))
	})

	if b.Ptero != nil {
		mux.HandleFunc("/action/", func(w http.ResponseWriter, r *http.Request) {

			path := strings.TrimPrefix(r.URL.Path, "/action/")
			parts := strings.Split(path, "/")

			if len(parts) < 2 {
				http.Error(w, "invalid action", 400)
				return
			}

			action := parts[0]
			serverID := parts[1]

			switch action {
			case "start":
				_ = b.Ptero.Power(serverID, flaphttp.Start)

			case "stop":
				_ = b.Ptero.Power(serverID, flaphttp.Stop)

			case "restart":
				_ = b.Ptero.Power(serverID, flaphttp.Restart)

			case "kill":
				_ = b.Ptero.Power(serverID, flaphttp.Kill)
			}

			fmt.Fprint(w, "ok")
		})
	}

	addr := fmt.Sprintf(":%d", b.port)
	fmt.Printf("🌐 Servidor HTTP iniciado em http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Erro ao iniciar servidor: %v\n", err)
	}
}

func stripActionTags(s string) string {
	re := regexp.MustCompile(`<getInfo\s+[^>]*\s*/>`)
	s = re.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

func (b *Build) ConvertTerminalButtons(content string) string {
	re := regexp.MustCompile(`<button\s+action="([^"]+)"\s*/?>`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}

		action := m[1]

		parts := strings.Split(action, "-")
		if len(parts) < 2 {
			return match
		}

		cmd := parts[0]
		id := parts[1]

		label := "action"

		switch cmd {
		case "restart":
			label = "🔄 reiniciar"
		case "start":
			label = "▶ iniciar"
		case "stop":
			label = "⛔ parar"
		case "kill":
			label = "💀 kill"
		}

		url := fmt.Sprintf("http://localhost:%d/action/%s/%s", b.port, cmd, id)

		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\  (%s)",
			url,
			label,
			url,
		)
	})
}

func (b *Build) ConvertButtons(content string) string {
	re := regexp.MustCompile(`<button\s+action="([^"]+)"\s*/?>`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}

		action := m[1]

		parts := strings.Split(action, "-")
		if len(parts) < 2 {
			return match
		}

		cmd := parts[0]
		id := parts[1]

		label := "action"

		switch cmd {
		case "restart":
			label = "🔄 reiniciar"
		case "start":
			label = "▶ iniciar"
		case "stop":
			label = "⛔ parar"
		case "kill":
			label = "💀 kill"
		}

		return fmt.Sprintf(
			`<a href="http://localhost:%d/action/%s/%s" style="
		padding:6px 10px;
		background:#7c3aed;
		color:white;
		border-radius:8px;
		text-decoration:none;
		display:inline-block;
		margin-top:6px;
	"> %s </a>`,
			b.port,
			cmd,
			id,
			label,
		)
	})
}
