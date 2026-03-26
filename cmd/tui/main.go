package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"maxbridge/internal/app"
	"maxbridge/internal/invites"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/storage"
	"maxbridge/internal/telegram"
	uitool "maxbridge/internal/tui"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.NewStore(context.Background(), cfg.DBDSN)
	if err != nil {
		fmt.Printf("db init error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	tg := telegram.NewClient(cfg.TelegramToken)
	mx := maxapi.NewClient(cfg.MaxAPIBaseURL, cfg.MaxToken)
	inv := invites.NewService(store, cfg.InviteHashPepper)
	service := uitool.NewAdminService(store, tg, mx, inv)

	model := uitool.NewModel(service)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("tui failed: %v\n", err)
		os.Exit(1)
	}
}
