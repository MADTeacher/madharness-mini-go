# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run` и `trace`. |
| `internal/config` | Загружает настройки из defaults, локального конфига, `.env` и переменных окружения. |
| `internal/agent` | Управляет режимами `ask` и `run`, хранит историю сообщений и вызывает инструменты. |
| `internal/model` | Делает HTTP-запрос в OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает общий контракт tool schemas, observations и registry. |
| `internal/tools/*tool` | Реализует файловые, поисковые, shell и patch handlers. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/trace` | Пишет JSONL-трассу и строит краткую сводку. |

## Поток `madharness-mini run`

1. `internal/cli` читает аргументы и создаёт `config.Config`.
2. `Config` применяет локальный конфиг, `.env` и переменные окружения.
3. `agent.Run()` создаёт `trace.Trace`, `model.Client`, `tools.Registry` и
   стартовые сообщения.
4. Модель получает `messages` и schemas доступных инструментов.
5. Если модель отвечает текстом, запуск завершается.
6. Если модель вызывает tool, `Registry` находит handler и выполняет его.
7. Handler проверяет путь или команду через `policy.Policy`.
8. Observation записывается в trace и возвращается модели как `role=tool`.
9. Цикл продолжается до финального ответа или лимита `max_turns`.

## Где смотреть при лабораторных

- CLI и пользовательский вывод: `internal/cli`.
- Настройки и значения по умолчанию: `internal/config`.
- Агентский цикл: `internal/agent`.
- Новые инструменты: `internal/tools` и подпакеты `internal/tools/*tool`.
- Безопасность путей и shell: `internal/policy`.
- Проверки trace: `internal/trace`.

Эта ветка намеренно держит всё близко к основному циклу. Следующие ветки будут
выносить проектные инструкции, контекст и расширения в отдельные слои.
