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