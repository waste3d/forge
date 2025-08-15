package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.com/waste3d/forge/ai/prompts"
	ai "github.com/waste3d/forge/openai"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Генерация forge конфига на базе вопросов",
	Long:  "Задаёт вопросы пользователю и генерирует forge.yaml для запуска окружения.",
	Run:   runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) {
	answers := make(map[string]string)

	reader := bufio.NewReader(os.Stdin)

	questions := []struct {
		key      string
		prompt   string
		defaultV string
	}{
		{"appName", "Название приложения", "my-app"},
		{"Stack", "Стек технологий (Go, Node.js, Python, Java и т.д.)", "Go"},
		{"Services", "Описание сервисов (через запятую: backend,frontend,worker)", "backend,frontend"},
		{"DB", "База данных (PostgreSQL, MySQL, Redis, None)", "PostgreSQL"},
		{"Dockerfile", "Генерировать Dockerfile для сервисов? (yes/no)", "yes"},
	}

	for _, q := range questions {
		fmt.Printf("%s [%s]: ", q.prompt, q.defaultV)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			text = q.defaultV
		}
		answers[q.key] = text
	}

	userInput := ""
	for k, v := range answers {
		userInput += fmt.Sprintf("%s: %s\n", k, v)
	}

	prompt := prompts.Init(userInput)

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = "Генерируем конфиг..."
	s.Start()

	aiResponse, err := ai.GenerateConfigWithAI(context.Background(), prompt)
	s.Stop()
	if err != nil {
		fmt.Println("Ошибка при генерации конфига:", err)
		return
	}

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	out, err := renderer.Render(aiResponse)
	if err != nil {
		fmt.Println(aiResponse)
	} else {
		fmt.Print(out)
	}

	// Сохраняем
	err = os.WriteFile("forge.yaml", []byte(aiResponse), 0644)
	if err != nil {
		errorLog(os.Stderr, "Ошибка сохранения forge.yaml: %v\n", err)
		os.Exit(1)
	}

	successLog("\n✅ forge.yaml успешно создан.\n")
}
