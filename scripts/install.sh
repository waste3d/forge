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