# Forge - Инструмент для Оркестрации Сред Разработки

![Статус проекта](https://img.shields.io/badge/status-в%20разработке-yellow)
![Go version](https://img.shields.io/badge/go-1.18+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

Forge — это инструмент командной строки, созданный для упрощения управления локальными средами разработки. Он использует Docker, но добавляет поверх этого умную логику и управление состоянием, чтобы сделать процесс более предсказуемым и надежным.

## Ключевые особенности

-   **Архитектура Клиент-Демон**: Легковесный клиент `forge` общается с фоновым демоном `forged`. Ваше окружение продолжает жить, даже если вы закрыли терминал.
-   **Управление состоянием**: Forge отслеживает все созданные ресурсы в локальной базе данных. Команда `forge down` чисто удаляет всё, что было создано.
-   **Установка одной командой**: Умный скрипт установки делает все за вас: собирает проект, находит лучшее место для установки и помогает настроить `PATH`.

## Установка

### Требования
1.  **Go** (версия 1.18 или выше) для сборки.
2.  **Docker** (установленный и запущенный).
3.  **Git** для клонирования репозитория.

### Установка одной командой

1.  Клонируйте репозиторий:
    ```bash
    git clone https://github.com/waste3d/forge.git
    cd forge
    ```

2.  Запустите скрипт установки:
    ```bash
    chmod +x scripts/install.sh
    ./scripts/install.sh
    ```

Скрипт автоматически соберет проект, установит бинарные файлы в `/usr/local/bin` (если есть права) или в `~/.local/bin` и подскажет, если нужно обновить ваш `PATH`.

## Использование

После успешной установки вы можете использовать `forge` из любой директории.

1.  **Зайдите в папку с вашим проектом** и создайте там файл `forge.yaml`:
    ```yaml
    version: 1
    appName: my-awesome-app
    databases:
      - name: main-db
        type: "postgres"
        version: "15"
        port: 54321
    ```

2.  **Запустите ваше окружение:**
    ```bash
    forge boot
    ```
    Эта команда автоматически запустит фоновый демон `forged` (если он еще не запущен) и развернет ваше окружение.

3.  **Остановите и удалите окружение:**
    ```bash
    forge down my-awesome-app
    ```

---

## Для разработчиков

### Ручная сборка
Если вы не хотите устанавливать Forge, а просто хотите собрать бинарные файлы, используйте скрипт `build.sh`.```bash
./scripts/build.sh
```
Результаты сборки для всех платформ появятся в директории `build/`.

<details>
<summary>Показать код скрипта scripts/install.sh</summary>

```bash
#!/bin/bash
#!/bin/bash
#
# Forge - Умный скрипт установки
#
# Этот скрипт выполняет следующие действия:
# 1. Определяет текущую ОС и архитектуру.
# 2. Собирает бинарные файлы 'forge' и 'forged' ТОЛЬКО для этой системы.
# 3. Определяет подходящую директорию для установки (/usr/local/bin или ~/.local/bin).
# 4. Создает эту директорию, если она не существует.
# 5. Копирует скомпилированные файлы.
# 6. Проверяет, находится ли директория в PATH, и выводит инструкцию, если нет.
#
set -e # Прекращать выполнение при любой ошибке

echo "🚀 Запуск установщика Forge..."

# --- Определение ОС и архитектуры ---
OS_NAME=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH_NAME=$(uname -m)
if [ "$ARCH_NAME" = "x86_64" ]; then
    ARCH_NAME="amd64"
fi

echo "📦 Сборка 'forge' и 'forged' для вашей системы ($OS_NAME/$ARCH_NAME)..."

# --- Сборка проекта для текущей системы ---
# Создаем временную папку для бинарных файлов
BUILD_TMP_DIR="./build_tmp"
rm -rf "$BUILD_TMP_DIR"
mkdir -p "$BUILD_TMP_DIR"

# Компилируем, указывая GOOS и GOARCH
GOOS=$OS_NAME GOARCH=$ARCH_NAME go build -o "$BUILD_TMP_DIR/forge" ./cmd/forge
GOOS=$OS_NAME GOARCH=$ARCH_NAME go build -o "$BUILD_TMP_DIR/forged" ./cmd/forged

echo "Сборка завершена."

# --- Определение директории для установки ---
INSTALL_DIR=""
USE_SUDO="false"

# Пробуем системный путь /usr/local/bin
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif sudo -v &>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
    USE_SUDO="true"
else
    # Если системный путь недоступен, используем локальный
    echo "Недостаточно прав для установки в /usr/local/bin. Устанавливаем локально."
    INSTALL_DIR="$HOME/.local/bin"
fi

echo "📂 Директория для установки: $INSTALL_DIR"

# --- Создание директории и копирование файлов ---
if [ "$USE_SUDO" = "true" ]; then
    echo "Требуются права администратора для создания директории и копирования файлов..."
    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$BUILD_TMP_DIR/forge" "$INSTALL_DIR/forge"
    sudo cp "$BUILD_TMP_DIR/forged" "$INSTALL_DIR/forged"
else
    mkdir -p "$INSTALL_DIR"
    cp "$BUILD_TMP_DIR/forge" "$INSTALL_DIR/forge"
    cp "$BUILD_TMP_DIR/forged" "$INSTALL_DIR/forged"
fi

# --- Очистка временных файлов ---
rm -rf "$BUILD_TMP_DIR"

echo "✅ 'forge' и 'forged' успешно установлены в $INSTALL_DIR"

# --- Проверка системного PATH ---
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "✅ Директория '$INSTALL_DIR' уже находится в вашем PATH."
    echo "🎉 Установка завершена! Можете использовать команду 'forge boot'."
    ;;
  *)
    echo
    echo "⚠️ ВНИМАНИЕ!"
    echo "Директория '$INSTALL_DIR' не найдена в вашем системном PATH."
    echo "Чтобы иметь возможность вызывать 'forge' из любого места, добавьте строку ниже"
    echo "в ваш конфигурационный файл оболочки (например, ~/.zshrc, ~/.bash_profile или ~/.profile):"
    echo
    echo "export PATH=\"$INSTALL_DIR:\$PATH\""
    echo
    echo "После этого перезапустите терминал или выполните команду 'source <имя_файла>'."
    ;;
esac

exit 0

</details>

<details>
<summary>Показать код скрипта scripts/build.sh</summary>

```bash
#!/bin/bash
#!/bin/bash

# Скрипт для кросс-компиляции проекта Forge
# для основных платформ: Linux, Windows, macOS.

# Прекращаем выполнение скрипта при любой ошибке.
set -e

# Переходим в корневую директорию проекта, чтобы пути ./cmd/... были правильными.
# `dirname "$0"` получает директорию, в которой находится сам скрипт (scripts).
# `/..` поднимается на один уровень вверх, в корень проекта.
cd "$(dirname "$0")/.."

# Директория для хранения всех сборок.
BUILD_DIR="build"

# Очищаем директорию от предыдущих сборок и создаем ее заново.
echo "Очистка директории '$BUILD_DIR'..."
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# --- Функция для сборки ---
# Принимает два аргумента:
# $1: GOOS (целевая операционная система, например, "linux")
# $2: GOARCH (целевая архитектура, например, "amd64")
build_target() {
    local os=$1
    local arch=$2
    local ext="" # Расширение файла (пустое по умолчанию)

    # Если собираем под Windows, добавляем расширение .exe
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    echo "Собираем для $os/$arch..."

    # Создаем поддиректорию для конкретной сборки, например, "build/forge-linux-amd64"
    local target_dir="$BUILD_DIR/forge-$os-$arch"
    mkdir -p $target_dir

    # Собираем бинарный файл демона 'forged'.
    # -o: флаг для указания пути и имени выходного файла.
    GOOS=$os GOARCH=$arch go build -o "$target_dir/forged$ext" ./cmd/forged

    # Собираем бинарный файл клиента 'forge'.
    GOOS=$os GOARCH=$arch go build -o "$target_dir/forge$ext" ./cmd/forge
}

# --- Цели для сборки ---
# Здесь мы вызываем нашу функцию для каждой комбинации ОС/архитектуры.

# Собираем для Linux (64-bit)
build_target "linux" "amd64"

# Собираем для Windows (64-bit)
build_target "windows" "amd64"

# Собираем для macOS (Intel 64-bit)
build_target "darwin" "amd64"

# Собираем для macOS (Apple Silicon ARM 64-bit)
build_target "darwin" "arm64"

echo ""
echo "✅ Сборка успешно завершена!"
echo "Бинарные файлы находятся в директории '$BUILD_DIR'."
```
</details>
