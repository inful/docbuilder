package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/app"
	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/service"
)

func main() {
	baseURL := os.Getenv("DOCBUILDER_TEMPLATE_BASE_URL")
	svc := service.NewTemplateService()
	model := app.NewModel(svc, baseURL)

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		_, _ = os.Stderr.WriteString("doctemplate failed: " + err.Error() + "\n")
		os.Exit(1)
	}
}
