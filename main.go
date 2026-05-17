package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	tea "github.com/charmbracelet/bubbletea"

	"ozorg.xyz/flapjack/build"
	"ozorg.xyz/flapjack/cache"
	flaphttp "ozorg.xyz/flapjack/http"
	"ozorg.xyz/flapjack/ia"
	"ozorg.xyz/flapjack/ia/prompt"
	"ozorg.xyz/flapjack/tui"
)

func main() {
	store, err := cache.Open()
	if err != nil {
		log.Fatalf("Erro ao abrir cache seguro: %v", err)
	}

	migrateFromEnv(store)

	ai := ia.NewIA(prompt.PROMPT, store)
	ptero := flaphttp.NewPterodactyl(store)
	builder := build.NewBuild(ai, ptero)

	if ptero != nil {
		go builder.StartServer()
	}

	m := tui.New(builder, store)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func migrateFromEnv(store *cache.Store) {
	_ = godotenv.Load()

	pairs := map[string]string{
		"GROQ_API_KEY":         os.Getenv("GROQ_API_KEY"),
		"PTERODACTYL_URL":      os.Getenv("PTERODACTYL_URL"),
		"PTERODACTYL_API_KEY":  os.Getenv("PTERODACTYL_API_KEY"),
	}

	for key, val := range pairs {
		if val == "" {
			continue
		}

		if _, err := store.Get(key); err != nil {
			if err := store.Set(key, val); err == nil {
				log.Printf("Cache: %s migrado do .env", key)
			}
		}
	}
}
