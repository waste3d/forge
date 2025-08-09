# Определяем имена бинарных файлов для легкого переиспользования
CLI_BINARY_NAME := forge
DAEMON_BINARY_NAME := forged
BINARY_DIR := ./bin

# .PHONY предотвращает конфликты с файлами, которые могут иметь те же имена,
# и гарантирует, что команда будет выполняться всегда.
.PHONY: all build install start-daemon stop-daemon restart-daemon run-cli lint test fmt clean help

# Команда по умолчанию, если просто запустить `make`
all: build

# -----------------
# ОСНОВНЫЕ КОМАНДЫ
# -----------------

# Собирает бинарные файлы в директорию ./bin
build:
	@echo "--> Собираем $(CLI_BINARY_NAME) и $(DAEMON_BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_DIR)/$(CLI_BINARY_NAME) ./cmd/forge/main.go
	@go build -o $(BINARY_DIR)/$(DAEMON_BINARY_NAME) ./cmd/forged/main.go
	@echo "Готово! Бинарные файлы в $(BINARY_DIR)/"

# Устанавливает бинарные файлы в системный $GOPATH/bin для глобального доступа
install:
	@echo "--> Устанавливаем бинарные файлы в $(GOPATH)/bin..."
	@go install ./cmd/forge/...
	@go install ./cmd/forged/...
	@echo "Успешно! Теперь вы можете вызывать 'forge' и 'forged' из любого места."

# -----------------
# КОМАНДЫ ДЛЯ РАЗРАБОТКИ ДЕМОНА
# -----------------

# Запускает скомпилированный демон в фоновом режиме
start-daemon: build
	@echo "--> Запускаем демона $(DAEMON_BINARY_NAME) в фоновом режиме..."
	@./$(BINARY_DIR)/$(DAEMON_BINARY_NAME) &

# Находит и останавливает запущенный процесс демона
stop-daemon:
	@echo "--> Останавливаем демона $(DAEMON_BINARY_NAME)..."
	@pkill -f "$(BINARY_DIR)/$(DAEMON_BINARY_NAME)" || echo "Демон не был запущен."

# Перезапускает демона: останавливает старый, собирает новый и запускает его.
# Идеально для разработки серверной части.
restart-daemon: stop-daemon build start-daemon
	@echo "--> Демон успешно перезапущен."

# -----------------
# КАЧЕСТВО КОДА
# -----------------

lint:
	@echo "--> Запускаем линтер..."
	@golangci-lint run

test:
	@echo "--> Запускаем тесты..."
	@go test ./... -v
	# @echo "MOCK TESTS!"

fmt:
	@echo "--> Форматируем код..."
	@go fmt ./...

# -----------------
# УПРАВЛЕНИЕ И ПОМОЩЬ
# -----------------

clean:
	@echo "--> Очищаем бинарные файлы и директории сборки..."
	@rm -rf $(BINARY_DIR)
	@rm -rf ./build

# Обновленная и более подробная помощь
help:
	@echo ""
	@echo "Управление проектом Forge"
	@echo "-------------------------"
	@echo "Основные команды:"
	@echo "  make build         - Собирает бинарные файлы в директорию ./bin"
	@echo "  make install       - Устанавливает бинарные файлы в систему (\$GOPATH/bin)"
	@echo ""
	@echo "Разработка демона:"
	@echo "  make start-daemon  - Запускает локальный демон в фоне"
	@echo "  make stop-daemon   - Останавливает локальный демон"
	@echo "  make restart-daemon- Пересобирает и перезапускает демон (основная команда для dev)"
	@echo ""
	@echo "Качество кода:"
	@echo "  make lint          - Запускает статический анализатор (golangci-lint)"
	@echo "  make test          - Запускает все тесты в проекте"
	@echo "  make fmt           - Форматирует весь код проекта"
	@echo ""
	@echo "Утилиты:"
	@echo "  make clean         - Удаляет скомпилированные бинарные файлы"
	@echo "  make help          - Показывает это сообщение"
	@echo ""