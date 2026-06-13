# madharness-mini-go

> Учебная ветка: `06-subagents`
>
> Тема главы: markdown-субагенты, роли и оркестрация внутри одного harness.
>
> В этой точке parent agent может получить инструмент `delegate_task`, запускать
> встроенных или project-local субагентов, передавать им ограниченный набор
> tools и получать отдельные дочерние trace-файлы.
>
> Лабораторные работы: [LABS.md](LABS.md)
> Предыдущая ветка: `05-mcp`
> Следующая ветка: `07-hooks`

`madharness-mini-go` - учебный минималистичный harness для работы кодирующего
ИИ-агента с локальным программным продуктом. Этот Go-порт показывает, как один
agent loop можно превратить в маленькую оркестрацию ролей, не добавляя внешних
runtime-зависимостей.

Проект написан для Go 1.25 и использует только стандартную библиотеку. Внутри
используется OpenAI-совместимый API `/chat/completions`, поэтому можно
подключить OpenRouter, KodikRouter, локальный совместимый сервер или другой
сервис с тем же форматом API.

## Что есть в этой ветке

- команды `init`, `ask`, `run`, `trace`, `skills` и `subagents`;
- проектные инструкции `AGENTS.md`;
- слой контекста с бюджетом и `context_report`;
- project-local Agent Skills и `activate_skill`;
- stdio MCP tools через `.madharness-mini/mcp.json`;
- встроенные роли `researcher`, `planner`, `implementer`, `reviewer`;
- project-local субагенты в `.madharness-mini/subagents/<name>.md`;
- режимы оркестрации `off`, `requested`, `auto`, `required`;
- инструменты `delegate_task` и, для разрешённых ролей, `ask_user`;
- отдельные дочерние traces для запусков субагентов.

В этой ветке ещё нет hooks. Здесь важны роли, profiles, allow-list tools и
передача результата обратно parent agent.

## Быстрый запуск

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

## Оркестрация

По умолчанию используется режим `auto`: parent agent видит `delegate_task`, но
может решить небольшую задачу сам.

Для одного запуска режим можно выбрать флагом:

```bash
go run ./cmd/madharness-mini run --no-orchestrate "Обнови короткий текст"
go run ./cmd/madharness-mini run --orchestration requested "Используй субагентов для ревью"
go run ./cmd/madharness-mini run --orchestrate-required "Разбей задачу между ролями"
```

В `.env` тот же выбор задаётся так:

```text
MADHARNESS_MINI_ORCHESTRATION_MODE=requested
```

Проверить встроенные и project-local роли:

```bash
go run ./cmd/madharness-mini subagents list
go run ./cmd/madharness-mini subagents validate
```

## Project-local субагент

Создайте `.madharness-mini/subagents/test-writer.md`:

```md
---
name: test-writer
description: Пишет минимальные Go-тесты для изменённого кода.
profile: writable
tools: ["list_files", "read_file", "search_code", "apply_patch", "write_file", "run_shell", "ask_user"]
max_turns: 10
---

Ты субагент test-writer.
Пиши маленькие тесты рядом с существующими Go-тестами.
Если не хватает требований, задай один короткий вопрос через ask_user.
```

`profile` не выдаёт полномочия сам по себе. Фактический набор доступных tools
задаётся списком `tools`.

## Документация ветки

- [Возможности ветки](docs/capabilities.md)
- [Структура кода](docs/code-overview.md)
- [Слой контекста](docs/context-layer.md)
- [Agent Skills](docs/agent-skills.md)
- [MCP](docs/mcp.md)
- [Субагенты](docs/subagents.md)
- [Инструмент apply_patch](docs/apply-patch.md)

## Разработка самого проекта

Если вы меняете код `madharness-mini-go`, запускайте проверки из корня
репозитория:

```bash
go test ./...
```

Быстрая ручная проверка CLI:

```bash
go run ./cmd/madharness-mini subagents list
go run ./cmd/madharness-mini subagents validate
```

## Что дальше

Следующая ветка `07-hooks` добавляет lifecycle hooks: локальные команды проекта
смогут наблюдать события harness и блокировать tool call до выполнения.

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
