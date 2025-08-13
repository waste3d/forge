package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.com/waste3d/forge/ai/prompts"
	ai "github.com/waste3d/forge/openai"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize [Dockerfile]",
	Short: "Оптимизирует Dockerfile с помощью AI",
	Long:  `Анализирует и оптимизирует Dockerfile с использованием искусственного интеллекта для улучшения производительности и безопасности.`,
	Args:  cobra.ExactArgs(1),
	Run:   runOptimize,
}

func init() {
	rootCmd.AddCommand(optimizeCmd)
}

func runOptimize(cmd *cobra.Command, args []string) {
	dockerfile := args[0]

	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		errorLog(os.Stderr, "Файл %s не найден\n", dockerfile)
		os.Exit(1)
	}

	ext := filepath.Ext(dockerfile)
	if ext != "" && ext != ".dockerfile" && ext != ".Dockerfile" {
		infoLog("Предупреждение: файл %s может не быть Dockerfile\n", dockerfile)
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Анализирую Dockerfile... (может занять до 30 секунд)"
	s.Start()

	content, err := os.ReadFile(dockerfile)
	if err != nil {
		errorLog(os.Stderr, "Ошибка чтения Dockerfile: %v\n", err)
		os.Exit(1)
	}

	if len(content) == 0 {
		errorLog(os.Stderr, "Dockerfile пуст\n")
		os.Exit(1)
	}

	prompt := prompts.DockerfilePrompt(string(content))

	aiResponse, err := ai.AnalyzeDockerfileWithAI(cmd.Context(), prompt)
	if err != nil {
		errorLog(os.Stderr, "Ошибка при анализе Dockerfile: %v\n", err)
		os.Exit(1)
	}

	s.Stop()

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		fmt.Println(aiResponse)
		return
	}

	out, err := renderer.Render(aiResponse)
	if err != nil {
		fmt.Println(aiResponse)
	} else {
		fmt.Print(out)
	}
}
