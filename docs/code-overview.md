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
| `internal/agent` | Собирает запуск: trace, model client, skills, subagents, MCP, context и registry. |
| `internal/model` | Вызывает OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает встроенные инструменты и общий `Registry`. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и краткую сводку. |

## Поток `run`

1. `internal/cli` создаёт `config.Config`.
2. `agent.RunWithOptions()` создаёт parent `Trace`, model client, discovery skills
   и discovery субагентов.
3. `subagents.ResolveOrchestrationMode()` вычисляет режим: `off`, `requested`,
   `auto` или `required`.
4. Skill catalog или явно выбранный skill добавляется в context.
5. `agentcontext.BaseContext()` добавляет встроенный prompt и корневой
   `AGENTS.md`.
6. `tools.Registry` получает встроенные tools, `activate_skill`, `delegate_task`
   при разрешённой оркестрации и MCP tools из `mcp.json`.
7. Parent model loop работает как обычно.
8. При `delegate_task` запускается дочерний loop с отдельным context, role
   prompt, allow-list tools и локальным trace.
9. Результат субагента возвращается parent как observation.
10. Если субагент вызвал `ask_user`, основной `run` печатает вопрос и
    завершается.

## Границы субагентов

Субагент не наследует полную историю parent. Он получает задачу делегации,
краткий parent context, свой markdown prompt и только разрешённые tools.

Встроенный `planner` дополнительно ограничен `.md` файлами, чтобы планирование
не превращалось в реализацию. Субагент не получает `delegate_task`, иначе
оркестрация могла бы стать рекурсивной и плохо наблюдаемой.

## MCP и субагенты

MCP остаётся источником инструментов для parent agent, как в ветке `05-mcp`.
Субагенты в базовой ветке получают встроенные tools и `ask_user`, но не запускают
отдельный MCP provider. Это держит дочерний запуск компактным и предсказуемым.

## Тестовое покрытие

Основные сценарии покрывают loader субагентов, CLI `subagents`, режимы
оркестрации, делегацию, role tools, `ask_user`, planner write-scope и дочерние
traces.
