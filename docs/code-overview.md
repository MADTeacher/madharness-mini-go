# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run`, `trace` и `skills`. |
| `internal/config` | Загружает настройки из defaults, локального конфига, `.env` и окружения. |
| `internal/agentcontext` | Собирает prompt fragments, задачу пользователя, историю и бюджет контекста. |
| `internal/skills` | Ищет skills, строит catalog, активирует `SKILL.md` и описывает resources. |
| `internal/agent` | Собирает запуск: trace, model client, skills discovery, context и registry. |
| `internal/model` | Вызывает OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает встроенные инструменты и provider `activate_skill`. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и краткую сводку. |

## Поток `run`

1. `internal/cli` создаёт `config.Config`.
2. `agent.Run()` создаёт `Trace`, `model.Client` и запускает discovery skills.
3. Если пользователь явно указал skill, он активируется до первого model call.
4. Если явного выбора нет, в контекст добавляется compact catalog, а в registry
   появляется tool `activate_skill`.
5. `agentcontext.BaseContext()` добавляет встроенный prompt и корневой
   `AGENTS.md`.
6. `tools.Registry` регистрирует встроенные инструменты и skill provider.
7. `agentcontext.Manager.Messages()` применяет бюджет и отдаёт messages.
8. Общий model/tool loop отправляет messages и tool schemas в модель.
9. Tool observation записывается в историю; служебный эффект активации skill
   добавляет durable fragment отдельно от observation.
10. Цикл продолжается до финального ответа или лимита `max_turns`.

## Границы skills

Skill не является плагином кода и не запускается сам. Он добавляет инструкции и
указывает на ресурсы внутри workspace. Если skill просит прочитать reference или
запустить script, модель всё равно должна использовать обычные инструменты:
`read_file` или `run_shell`.

Это делает skills хорошей учебной ступенью: студент видит, как расширяется
контекст, не смешивая это с внешними процессами или новыми протоколами.

## Данные и безопасность

`Config.Root` задаёт workspace. Все файловые инструменты проходят через
`Policy.SafePath()`, поэтому относительные пути не выходят за рабочую папку, а
защищённые пути из `protected_paths` блокируются.

Discovery skills использует отдельный `Policy.SkillRoot()`: skill-каталоги
должны оставаться внутри workspace, но не проходят через `protected_paths`,
потому что это фиксированные служебные roots, а не произвольный пользовательский
путь.

## Тестовое покрытие

Основные сценарии лежат в `internal/skills/*_test.go` и соседних пакетах:
discovery, frontmatter, explicit selection, compact catalog, activation,
resources, контекст, инструменты, CLI и trace.
