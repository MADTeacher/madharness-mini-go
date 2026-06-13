# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run` и `trace`. |
| `internal/config` | Собирает настройки из defaults, локального конфига, `.env` и переменных окружения. |
| `internal/agentcontext` | Собирает фрагменты, задачу, историю и бюджет контекста. |
| `internal/agent` | Остаётся публичным фасадом режимов `ask` и `run`. |
| `internal/model` | Делает HTTP-запрос в OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает общий контракт tool schemas, observations и registry. |
| `internal/tools/imagetool` | Проверяет изображения и готовит скрытое vision-вложение. |
| `internal/tools/*tool` | Реализует файловые, поисковые, shell и patch handlers. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/trace` | Пишет JSONL-трассу и строит краткую сводку. |

## Поток `run`

1. `internal/cli` читает аргументы и создаёт `config.Config`.
2. `agent.Run()` создаёт `Trace`, `model.Client`, `tools.Registry` и
   `agentcontext.Manager`.
3. `agentcontext.BaseContext()` добавляет встроенный prompt и корневой
   `AGENTS.md` как закреплённые фрагменты.
4. `Manager.Messages()` собирает system/user/history и применяет бюджет.
5. Общий model/tool loop отправляет messages и tool schemas в модель.
6. Если модель отвечает текстом, запуск завершается.
7. Если модель вызывает tool, registry выполняет handler.
8. Observation пишется в trace и в историю контекста.
9. Цикл продолжается до финального ответа или лимита `max_turns`.

## Данные и границы

`Config.Root` задаёт workspace. Все файловые инструменты проходят через
`Policy.SafePath()`, поэтому относительные пути не выходят за рабочую папку, а
защищённые пути из `protected_paths` блокируются.

Ответы инструментов имеют единый вид: успешные создаются через `tools.OK()`,
ошибочные через `tools.Fail()`. Модель получает observation с полями `ok`,
`tool` и `summary`, а дополнительные данные зависят от конкретного инструмента.

Слой контекста не вызывает handlers и не проверяет безопасность путей. Его роль
уже: хранить то, что будет отправлено модели, и объяснять через
`context_report`, почему именно этот набор messages поместился в запрос.

## Тестовое покрытие

Основные сценарии лежат в `internal/*/*_test.go`: конфигурация, проектные
инструкции, policy, инструменты, model loop и слой контекста.
