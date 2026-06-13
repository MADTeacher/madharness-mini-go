# madharness-mini-go

> Учебная ветка: `07-hooks`
>
> Тема главы: lifecycle hooks как проектный слой аудита и блокировки действий
> агента.
>
> В этой точке harness умеет читать `.madharness-mini/hooks.json`, отправлять
> JSON-события локальным command hooks, писать hook-аудит в trace и блокировать
> `before_tool_call` до запуска handler-а.
>
> Лабораторные работы: [LABS.md](LABS.md)
> Предыдущая ветка: `06-subagents`

`madharness-mini-go` - учебный минималистичный harness для работы кодирующего
ИИ-агента с локальным программным продуктом. Эта ветка показывает полный набор
механизмов Go-порта: workspace tools, проектные инструкции, context layer,
Agent Skills, MCP, субагентов и hooks.

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
- markdown-субагенты, `delegate_task`, `ask_user` и дочерние traces;
- lifecycle hooks из `.madharness-mini/hooks.json`;
- redaction payload перед передачей hook-команде;
- блокировка tool call через `before_tool_call`.

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

Запустите агентский режим:

```bash
go run ./cmd/madharness-mini run "Найди команду для запуска тестов и объясни, что она проверяет"
```

## Минимальный hook

Создайте `.madharness-mini/hooks.json`:

```json
{
  "hooks": [
    {
      "id": "deny-shell",
      "event": "before_tool_call",
      "match": { "tool": "run_shell" },
      "command": "python3",
      "args": ["scripts/hooks/deny_shell.py"],
      "cwd": ".",
      "timeout_seconds": 3
    }
  ]
}
```

Hook-команда получает событие в stdin. Если она печатает:

```json
{ "ok": false, "block": "shell запрещён правилами проекта" }
```

handler инструмента не запускается, а модель получает обычное fail-observation.
Остальные события нужны для аудита и диагностики; сейчас блокировать действие
может только `before_tool_call`.

## Документация ветки

- [Возможности ветки](docs/capabilities.md)
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
go run ./cmd/madharness-mini run "Объясни, какие hooks подключены в этом проекте"
go run ./cmd/madharness-mini trace <trace-id>
```

## Как читать эту ветку

Это самая полная учебная точка Go-порта. Если вы пришли из книги или курса,
сначала посмотрите [LABS.md](LABS.md), затем откройте
[docs/README.md](docs/README.md) и переходите в конкретный документ по
механизму, который сейчас изучаете.

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
