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
| `internal/events` | Публикует lifecycle-события подписчикам и сохраняет trace-события через trace subscriber. |
| `internal/hooks` | Читает hooks config, запускает enforce/observe command handlers, делает redaction и пишет hook events в trace. |
| `internal/approval` | Обрабатывает эскалируемые policy-отказы через hooks, YOLO, CLI prompt или config-deny. |
| `internal/agent` | Собирает запуск и agent sessions: trace, hooks, model client, skills, subagents, MCP, context и registry. |
| `internal/agent/turnexec` | Планирует read-only и delegate batches и выполняет их конкурентно внутри одного turn-а. |
| `internal/model` | Вызывает OpenAI-совместимый `/chat/completions`. |
| `internal/processes` | Управляет долгоживущими shell-процессами одного `run`. |
| `internal/tools` | Описывает встроенные инструменты и общий `Registry`. |
| `internal/workspace` | Координирует path-level и global locks для tools разных agent sessions. |
| `internal/policy` | Проверяет workspace-границы, protected paths и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и краткую сводку. |

## Поток `run`

1. `internal/cli` создаёт `config.Config`.
2. `agent.RunWithOptions()` создаёт parent `Trace`, `hooks.Manager`,
   `events.Bus`, общий process manager и root `Session`.
3. Event bus публикует `session_start`; trace уже содержит начальный
   `session_start` от `trace.New()`, hooks получают project payload.
4. Загружаются skills, subagents и MCP providers.
5. `agentcontext.BaseContext()` добавляет встроенный prompt, корневой
   `AGENTS.md`, catalog skills и историю.
6. Перед model call публикуется `before_model_call`, после ответа -
   `after_model_call`; trace и hooks получают одно lifecycle-событие через
   разных подписчиков.
7. Если модель вызывает tools, `runModelLoop()` синхронно публикует
   `before_tool_call` для каждого call.
8. Если enforce hook блокирует `before_tool_call`, handler не запускается, а
   модель получает fail-observation. Observe hooks не блокируют.
9. Если блокировки нет, `turnexec` параллелит соседние read-only tools и
   соседние `delegate_task` в разных batches, а write/shell/state/MCP calls
   оставляет барьерами.
10. Если tool упёрся в эскалируемый policy-отказ, `internal/approval`
    публикует `approval_request` и `approval_decision`; `approval_request`
    может быть заблокирован enforce hook-ом до CLI prompt.
11. После observation вызывается `after_tool_call`, а commit в context идёт в
    исходном порядке `tool_calls`.
12. Перед выходом из `run` общий process manager останавливает оставшиеся
    managed shell processes.
13. При нормальном завершении hooks получают `session_end`, при ошибке -
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

Главный агент и каждый субагент выполняются как отдельные `agent.Session`.
Сессии разделяют model client, workspace scheduler и guarded approval prompter,
managed shell processes, но имеют отдельные context, registry, event bus и
trace.

## Тестовое покрытие

Основные hook-сценарии покрывают `internal/hooks` и `internal/agent`: no-op без
конфига, валидацию `hooks.json`, redaction, безопасное окружение, блокировку
`before_tool_call`, падение hook-процесса и дочерний trace субагента. Остальные
тесты покрывают context, skills, MCP, subagents, policy и базовые tools.
