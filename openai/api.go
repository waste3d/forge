package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/waste3d/forge/cmd/forge/cli/helpers"
)

func AnalyzeLogsWithAI(ctx context.Context, collectedLogs map[string][]string) (string, error) {
	apiKey := os.Getenv("AI_API_KEY")
	if apiKey == "" {
		apiKey = "YOUR_OPENROUTER_API_KEY"
	}

	var logBuilder strings.Builder
	for serviceName, logs := range collectedLogs {
		for _, logLine := range logs {
			logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", serviceName, logLine))
		}
	}

	prompt := helpers.BuildPrompt(logBuilder.String())

	reqBody := map[string]interface{}{
		"model": "openai/gpt-oss-20b:free", // free model
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
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
		return "", fmt.Errorf("ошибка отправки запроса к DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка от DeepSeek API: %s", string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "ИИ не вернул ответа", nil
}
