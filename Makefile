.PHONY: build
build:
	@echo "--> Собираем forge и forged..."
	@go build -o forge ./cmd/forge/main.go
	@go build -o forged ./cmd/forged/main.go
	@echo "Готово! Бинарные файлы в ./bin/"

.PHONY: lint
lint:
	@echo "--> Запускаем линтер..."
	@golangci-lint run

.PHONY: test
test:
	@echo "--> Запускаем тесты..."
	@echo "MOCK TESTS!"

.PHONY: fmt
fmt:
	@echo "--> Форматируем код..."
	@gofmt -w .

.PHONY: clean
clean:
	@echo "--> Очищаем бинарные файлы..."
	@rm -rf ./bin
	@rm -rf ./build

.PHONY: help
help:
	@echo "--> Помощь по командам..."
	@echo "make build - Собирает forge и forged"
	@echo "make lint - Запускает линтер"
	@echo "make test - Запускает тесты"
	@echo "make fmt - Форматирует код"
	@echo "make clean - Очищает бинарные файлы"
	@echo "make help - Показывает эту помощь"