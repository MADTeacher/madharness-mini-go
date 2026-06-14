# madharness-mini-go

`madharness-mini-go` — экспериментальный минималистичный harness 

Проект написан для Go 1.25 и использует только стандартную библиотеку. Внутри используется OpenAI-совместимый API `/chat/completions`, поэтому можно подключить OpenRouter, KodikRouter, локальный совместимый сервер или другой сервис с тем же форматом API.

## Быстрый старт финальной версии

Перейдите в корень проекта:

```bash
cd /path/to/madharness-mini-go
```

Создайте локальную настройку:

```bash
go run ./cmd/madharness-mini init \
  --base-url https://openrouter.ai/api/v1 \
  --model deepseek/deepseek-v4-flash \
  --api-key "ключ-доступа-openrouter"
```

Если ключ уже лежит в `.env`, достаточно:

```bash
go run ./cmd/madharness-mini init --no-prompt
```

Задайте вопрос без инструментов:

```bash
go run ./cmd/madharness-mini ask "Объясни, что делает этот проект"
```

Запустите агентский режим:

```bash
go run ./cmd/madharness-mini run "Найди команду для запуска тестов и объясни, что она проверяет"
```

Для сценариев с долгими dev-процессами агенту доступны managed shell tools:
`start_shell`, `shell_status` и `stop_shell`. Например, задача может попросить
поднять backend и frontend, дождаться startup-строк через `ready_pattern`, а
затем проверить клиентскую часть через подключённые MCP tools:

```bash
go run ./cmd/madharness-mini run \
  "Запусти backend и frontend как managed shell processes, дождись готовности клиента и проверь основной экран через MCP"
```

Такие процессы живут только внутри текущего `run` и автоматически завершаются
при выходе harness.

Для параллельного выполнения соседних read-only и MCP tools внутри одного turn-а:

```bash
go run ./cmd/madharness-mini run --max-parallel-tool-calls 2 "Найди документацию про конкурентность"
```

Для параллельного запуска соседних `delegate_task`:

```bash
go run ./cmd/madharness-mini run --max-parallel-subagents 2 --orchestrate "Поручи двум субагентам независимую проверку"
```

Для явного подтверждения эскалируемого policy-отказа в CLI:

```bash
go run ./cmd/madharness-mini run --approval ask "Запусти нужную проверку"
```

YOLO-режим автоматически подтверждает такие отказы внутри workspace:

```bash
go run ./cmd/madharness-mini run --yolo "Проверь проект максимально свободно"
```

Посмотрите трассу:

```bash
go run ./cmd/madharness-mini trace <trace-id>
```

Новые запуски пишутся в `.madharness-mini/traces/<trace-id>/<trace-id>.jsonl`;
старые плоские файлы `.madharness-mini/traces/<trace-id>.jsonl` команда
`trace` тоже читает.

## Финальная документация

- [Возможности полной версии](docs/capabilities.md)
- [Структура кода](docs/code-overview.md)
- [Слой контекста](docs/context-layer.md)
- [Agent Skills](docs/agent-skills.md)
- [MCP](docs/mcp.md)
- [Субагенты](docs/subagents.md)
- [Hooks](docs/hooks.md)
- [Инструмент apply_patch](docs/apply-patch.md)

## Разработка самого проекта

Если вы меняете код `madharness-mini-go`, запускайте проверки из корня
репозитория:

```bash
go test ./...
```

Быстрая ручная проверка CLI:

```bash
go run ./cmd/madharness-mini ask "Объясни, что делает этот проект"
go run ./cmd/madharness-mini run "Найди команду для запуска тестов и объясни, что она проверяет"
```

## Лицензирование

Проект использует раздельную лицензионную модель:

- код распространяется по PolyForm Noncommercial License 1.0.0;
- учебные материалы распространяются по Creative Commons
  Attribution-NonCommercial-ShareAlike 4.0 International.

Некоммерческое самообучение, академическое преподавание и исследовательское
использование разрешены на условиях соответствующих лицензий. Коммерческое
использование кода или материалов требует предварительного письменного
разрешения правообладателя.

Полные тексты и русские версии: [LICENSE.md](LICENSE.md).
