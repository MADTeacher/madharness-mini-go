# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run`, `trace` и `skills`. |
| `internal/config` | Загружает настройки из defaults, локального конфига, `.env` и окружения. |
| `internal/agentcontext` | Собирает prompt fragments, задачу пользователя, историю и бюджет контекста. |
| `internal/skills` | Ищет skills, строит catalog, активирует `SKILL.md` и описывает resources. |
| `internal/mcp` | Реализует stdio MCP client, JSON-RPC, config loader, provider и result adapter. |
| `internal/agent` | Собирает запуск: trace, model client, skills discovery, MCP providers, context и registry. |
| `internal/model` | Вызывает OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает встроенные инструменты и общий `Registry`. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и краткую сводку. |

## Поток `run`

1. `internal/cli` создаёт `config.Config`.
2. `agent.Run()` создаёт `Trace`, `model.Client` и запускает discovery skills.
3. Skill catalog или явно выбранный skill добавляется в context.
4. `agentcontext.BaseContext()` добавляет встроенный prompt и корневой
   `AGENTS.md`.
5. `tools.Registry` получает встроенные tools, `activate_skill` и MCP tools из
   `.madharness-mini/mcp.json`, если файл есть.
6. MCP provider запускает включённые stdio servers, вызывает `initialize` и
   `tools/list`, затем создаёт `tools.Spec` для каждого внешнего tool.
7. `agentcontext.Manager.Messages()` применяет бюджет.
8. Общий model/tool loop отправляет messages и tool schemas в модель.
9. При MCP tool call provider отправляет `tools/call` внешнему серверу и
   превращает ответ в обычное observation.
10. `tools.Registry.Close()` закрывает MCP-процессы через `defer`.

## Границы MCP

MCP-сервер — доверенный локальный процесс, но harness всё равно задаёт явные
границы: запуск без shell, workspace-relative `cwd`, безопасное окружение и
обычный формат observation. Это учебно важно: внешний tool не должен ломать
модельный протокол и не должен незаметно получать секреты API.

## Данные и безопасность

`Config.Root` задаёт workspace. Все файловые инструменты проходят через
`Policy.SafePath()`, поэтому относительные пути не выходят за рабочую папку, а
защищённые пути из `protected_paths` блокируются.

MCP `cwd` тоже проходит через `Policy.SafePath()`. Переменные окружения
`MADHARNESS_MINI_*` не наследуются внешними MCP-процессами автоматически:
сервер получает только безопасный минимум системного окружения и явный `env` из
`mcp.json`.

## Тестовое покрытие

Основные сценарии MCP лежат в `internal/mcp/mcp_test.go`: config, stdio
protocol, provider, result conversion, безопасное окружение и закрытие
процессов. Остальные тесты продолжают проверять config, context, skills,
policy, tools, agent loop и trace.
