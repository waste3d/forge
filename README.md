# Forge — Оркестратор Локальных Сред Разработки

![Статус проекта](https://img.shields.io/badge/status-в%20разработке-yellow)
![Go version](https://img.shields.io/badge/go-1.18+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

**Forge** — это инструмент командной строки для быстрого и предсказуемого развёртывания локальных сред разработки. Он использует **Docker** как основу, добавляя поверх умную логику, управление состоянием, постоянный демон и интеграцию с ИИ для анализа логов.

---

## 🚀 Основные возможности

* **Архитектура клиент–демон**
  Лёгкий клиент `forge` общается с фоновым демоном `forged`, который продолжает работать даже после закрытия терминала.
* **Чистое управление состоянием**
  Все созданные ресурсы отслеживаются и удаляются командой `forge down`.
* **Интеграция с ИИ**
  Флаг `--ai` в `forge logs` позволяет автоматически проанализировать логи и найти первопричину проблем.
* **Интерактивный доступ в контейнеры**
  Команда `forge exec` с поддержкой `-it` для интерактивных сессий.
* **Простая установка**
  Скрипт установки соберёт бинарники, выберет директорию и обновит `PATH`.
* **Кросс-компиляция**
  Скрипт `build.sh` собирает проект для Linux, Windows и macOS.

---

## 📦 Установка

### Требования

1. **Go** ≥ 1.18
2. **Docker** (установленный и запущенный)
3. **Git** для клонирования репозитория

### Установка одной командой

```bash
git clone https://github.com/waste3d/forge.git
cd forge
chmod +x scripts/install.sh
./scripts/install.sh
```

---

## ⚡ Быстрый старт

1. **Создайте конфиг `forge.yaml`** в корне проекта:

```yaml
version: 1
appName: my-awesome-app
databases:
  - name: main-db
    type: postgres
    version: "15"
    port: 54321
services:
  - name: backend
    path: ./backend
    dockerfile: Dockerfile
    ports:
      - "8080:8080"
```

2. **Запустите окружение**:

```bash
forge up
```

3. **Посмотрите логи**:

```bash
forge logs my-awesome-app
# или с анализом через ИИ:
forge logs my-awesome-app --ai
```

4. **Выполните команду внутри контейнера**:

```bash
forge exec my-awesome-app backend -it -- /bin/bash
```

5. **Остановите и удалите окружение**:

```bash
forge down my-awesome-app
```

---

## 🛠 Команды CLI

| Команда                                       | Описание                                   |
| --------------------------------------------- | ------------------------------------------ |
| `forge up`                                    | Запуск окружения из `forge.yaml`           |
| `forge down [appName]`                        | Остановка и удаление окружения             |
| `forge logs [appName] [serviceName]`          | Просмотр логов (флаги: `--follow`, `--ai`) |
| `forge ps [appName]`                          | Список запущенных сервисов                 |
| `forge exec <appName> <serviceName> -- <cmd>` | Выполнить команду в контейнере             |
| `forge system start/stop/status`              | Управление демоном `forged`                |
| `forge version`                               | Показать версию                            |

---

## 👨‍💻 Для разработчиков

### Ручная сборка

```bash
./scripts/build.sh
```

Результаты будут в `build/`.

### Кросс-компиляция

`build.sh` собирает для Linux, Windows и macOS (Intel и ARM).

---

## 📄 Лицензия

![MIT](https://github.com/waste3d/forge/blob/master/LICENSE)