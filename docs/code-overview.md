# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run`, `trace`, `skills` и `subagents`. |
| `internal/config` | Загружает настройки из defaults, локального конфига, `.env` и окружения. |
| `internal/agentcontext` | Собирает prompt fragments, задачу пользователя, историю и бюджет контекста. |
| `internal/skills` | Ищет skills, строит catalog, активирует `SKILL.md` и описывает resources. |
| `internal/mcp` | Реализует stdio MCP client, JSON-RPC, config loader, provider и result adapter. |
| `internal/subagents` | Загружает роли, вычисляет режим оркестрации, описывает tools субагентов и сводит дочерние traces. |
| `internal/hooks` | Читает hooks config, запускает command handlers, делает redaction и пишет hook events в trace. |
| `internal/agent` | Собирает запуск: trace, hooks, model client, skills, subagents, MCP, context и registry. |
| `internal/agent/turnexec` | Планирует read-only tool batches и выполняет их конкурентно внутри одного turn-а. |
| `internal/model` | Вызывает OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает встроенные инструменты и общий `Registry`. |
| `internal/policy` | Проверяет workspace-границы, protected paths и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и краткую сводку. |

## Поток `run`

1. `internal/cli` создаёт `config.Config`.
2. `agent.RunWithOptions()` создаёт parent `Trace` и `hooks.Manager`.
3. Hooks получают `session_start`.
4. Загружаются skills, subagents и MCP providers.
5. `agentcontext.BaseContext()` добавляет встроенный prompt, корневой
   `AGENTS.md`, catalog skills и историю.
6. Перед model call вызывается `before_model_call`, после ответа -
   `after_model_call`.
7. Если модель вызывает tools, `runModelLoop()` синхронно вызывает
   `before_tool_call` для каждого call.
8. Если hook блокирует действие, handler не запускается, а модель получает
   fail-observation.
9. Если блокировки нет, `turnexec` параллелит только соседние read-only tools,
   а write/shell/state/delegate/MCP calls оставляет барьерами.
10. После observation вызывается `after_tool_call`, а commit в context идёт в
    исходном порядке `tool_calls`.
11. При нормальном завершении hooks получают `session_end`, при ошибке -
    `session_error`.

## Границы hooks

Hook-команда - доверенный локальный код проекта, но harness удерживает несколько
границ: запуск без shell, workspace-relative `cwd`, безопасное окружение,
redaction payload и запись ошибок в trace. Hooks добавляют проектную политику,
но не отменяют `Policy.SafePath()` и shell-проверки.

## Субагенты

Субагент не наследует полную историю parent. Он получает задачу делегации,
краткий parent context, свой markdown prompt и только разрешённые tools.

При `delegate_task` дочерний запуск получает тот же набор hook providers, но с
дочерней трассой. Поэтому `match: { "kind": "subagent" }` позволяет писать
правила только для субагентов.

## Тестовое покрытие

Основные hook-сценарии покрывают `internal/hooks` и `internal/agent`: no-op без
конфига, валидацию `hooks.json`, redaction, безопасное окружение, блокировку
`before_tool_call`, падение hook-процесса и дочерний trace субагента. Остальные
тесты покрывают context, skills, MCP, subagents, policy и базовые tools.
