#!/bin/bash

# Скрипт для кросс-компиляции проекта Forge
# для основных платформ: Linux, Windows, macOS.

# Прекращаем выполнение скрипта при любой ошибке
set -e

# Директория для хранения сборок
BUILD_DIR="build"

# Очищаем и создаем директорию для сборок
echo "Очистка директории $BUILD_DIR..."
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# --- Функция для сборки ---
build_target() {
    local os=$1
    local arch=$2
    local ext=""

    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    echo "Собираем для $os/$arch..."

    # Создаем поддиректорию для конкретной сборки
    local target_dir="$BUILD_DIR/forge-$os-$arch"
    mkdir -p $target_dir

    # Собираем демон
    GOOS=$os GOARCH=$arch go build -o "$target_dir/forged$ext" ./cmd/forged

    # Собираем клиент
    GOOS=$os GOARCH=$arch go build -o "$target_dir/forge$ext" ./cmd/forge
}

# --- Цели для сборки ---
build_target "linux" "amd64"
build_target "windows" "amd64"
build_target "darwin" "amd64"  # macOS Intel
build_target "darwin" "arm64"  # macOS Apple Silicon

echo ""
echo "Сборка успешно завершена!"
echo "Бинарные файлы находятся в директории '$BUILD_DIR'."