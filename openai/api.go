package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/waste3d/forge/ai/prompts"
)

type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func makeOpenRouterRequest(ctx context.Context, prompt string, maxTokens int) (string, error) {
	apiKey := os.Getenv("AI_API_KEY")
	if apiKey == "" || apiKey == "YOUR_OPENROUTER_API_KEY" {
		return "", fmt.Errorf("не установлен API ключ. Установите переменную окружения AI_API_KEY")
	}

	reqBody := map[string]interface{}{
		"model": "openai/gpt-oss-20b:free",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка отправки запроса к OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка от OpenRouter API (статус %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "ИИ не вернул ответа", nil
}

func AnalyzeLogsWithAI(ctx context.Context, collectedLogs map[string][]string) (string, error) {
	var logBuilder strings.Builder
	for serviceName, logs := range collectedLogs {
		for _, logLine := range logs {
			logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", serviceName, logLine))
		}
	}

	prompt := prompts.LogsPrompt(logBuilder.String())
	return makeOpenRouterRequest(ctx, prompt, 1500)
}

func AnalyzeDockerfileWithAI(ctx context.Context, prompt string) (string, error) {
	return makeOpenRouterRequest(ctx, prompt, 1500)
}
