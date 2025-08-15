package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
}
