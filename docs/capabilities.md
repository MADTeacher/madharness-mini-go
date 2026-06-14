# Возможности ветки

`07-hooks` - финальная учебная точка Go-порта. Здесь собраны механизмы
предыдущих веток и добавлен проектный слой lifecycle hooks вокруг agent loop.

## Команды CLI

| Команда | Что делает |
| --- | --- |
| `madharness-mini init` | Создаёт или обновляет `.madharness-mini/config.json`. |
| `madharness-mini ask "..."` | Отправляет один запрос модели без tools. |
| `madharness-mini run "..."` | Запускает agent loop с tools, context, skills, MCP, subagents и hooks. |
| `madharness-mini trace <id>` | Показывает краткую сводку JSONL-трассы. |
| `madharness-mini skills list/show/validate` | Диагностирует project-local Agent Skills. |
| `madharness-mini subagents list/show/validate` | Диагностирует встроенных и project-local субагентов. |

При локальной разработке команда запускается как `go run ./cmd/madharness-mini`.

Для одного запуска `run` можно включить ограниченный параллелизм read-only
tools и соседних делегаций:

```bash
go run ./cmd/madharness-mini run --max-parallel-tool-calls 2 "..."
go run ./cmd/madharness-mini run --max-parallel-subagents 2 --orchestrate "..."
```

Эскалируемые policy-отказы можно оставить в безопасном режиме `deny` или
спросить пользователя в CLI:

```bash
go run ./cmd/madharness-mini run --approval ask "..."
```

`--yolo` автоматически подтверждает эскалируемые отказы внутри workspace, но не
разрешает файловым tools выходить за `workspace_root`.

## Hooks

Hooks настраиваются в:

```text
.madharness-mini/hooks.json
```

Hook - это локальная command handler, которая получает JSON-событие в stdin.
События: `session_start`, `before_model_call`, `after_model_call`,
`before_tool_call`, `after_tool_call`, `approval_request`,
`approval_decision`, `session_end`, `session_error`.

По умолчанию hook работает в режиме `enforce`. Также можно указать
`"mode": "observe"`: такой hook выполняется через очередь и никогда не
блокирует действие.

Только enforce hook на `before_tool_call` или `approval_request` может
остановить действие. Для блокировки hook возвращает JSON:

```json
{ "ok": false, "block": "причина блокировки" }
```

Harness не запускает tool handler и возвращает модели обычное observation
`ok=false`.

## Остальные механизмы

Ветка сохраняет:

- `AGENTS.md` как проектные инструкции;
- context budget и `context_report`;
- `read_image` для vision input;
- Agent Skills и `activate_skill`;
- stdio MCP tools;
- markdown-субагентов, `delegate_task`, `ask_user` и дочерние traces;
- agent sessions для root/subagent loop-ов и общий workspace scheduler;
- базовые workspace tools: `list_files`, `read_file`, `search_code`,
  `write_file`, `apply_patch`, `run_shell`.

## Безопасность hooks

- `command` и `args` запускаются без shell;
- `cwd` должен быть внутри workspace и проходить общую policy;
- payload обрезается и проходит redaction очевидных секретов;
- `MADHARNESS_MINI_*` не наследуются hook-командой автоматически;
- ошибки audit hooks пишутся в trace и обычно не ломают запуск;
- блокировка имеет смысл только для `before_tool_call` и `approval_request`.

Hooks не заменяют `Policy`. Они добавляют проектные правила поверх общей защиты
workspace, shell-команд и protected paths.
