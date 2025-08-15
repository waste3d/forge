package prompts

import "fmt"

func Init(userInput string) string {
	return fmt.Sprintf(`
Ты — эксперт по DevOps и Go.
На основе следующей информации сгенерируй корректный forge.yaml для микросервисного приложения.
Входные данные:
---
%s
---
Требования:
- Только YAML, без пояснений
- Конфигурация должна быть готова к запуску через "forge up"
- Если база данных не "None", добавь соответствующий сервис
- Если dockerfile = "yes", добавь в конфиг путь к Dockerfile для каждого сервиса
`, userInput)
}
