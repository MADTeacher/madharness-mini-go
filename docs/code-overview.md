# Структура кода

## Основные пакеты

| Пакет | За что отвечает |
| --- | --- |
| `cmd/madharness-mini` | Минимальная точка входа CLI. |
| `internal/cli` | Разбирает команды `init`, `ask`, `run` и `trace`. |
| `internal/config` | Собирает настройки из defaults, локального конфига, `.env` и переменных окружения. |
| `internal/instructions` | Загружает корневой `AGENTS.md` с лимитом размера. |
| `internal/agent` | Собирает стартовые сообщения, управляет `ask` и циклом model/tool calls. |
| `internal/model` | Делает HTTP-запрос в OpenAI-совместимый `/chat/completions`. |
| `internal/tools` | Описывает общий контракт tool schemas, observations и registry. |
| `internal/tools/imagetool` | Проверяет изображения и готовит скрытое vision-вложение. |
| `internal/tools/*tool` | Реализует файловые, поисковые, shell и patch handlers. |
| `internal/policy` | Проверяет workspace-границы, защищённые пути и shell-команды. |
| `internal/prompt` | Загружает встроенный системный prompt через `embed`. |
| `internal/trace` | Пишет JSONL-трассу и строит краткую сводку. |

## Поток `ask`

1. `internal/cli` читает аргументы и создаёт `config.Config`.
2. `Config` применяет локальный конфиг, `.env` и переменные окружения.
3. `agent.BaseMessages()` берёт встроенный system prompt.
4. `instructions.LoadProject()` добавляет найденный корневой `AGENTS.md`.
5. `model.Client` отправляет один запрос без tools.
6. Ответ и trace возвращаются пользователю.

## Поток `run`

1. `agent.Run()` создаёт `Trace`, `model.Client`, `tools.Registry` и стартовые сообщения.
2. Модель получает сообщения и schemas инструментов.
3. Если модель вызывает tool, registry выполняет handler.
4. Observation пишется в trace и возвращается модели как `role=tool`.
5. Для `read_image` registry отделяет скрытое follow-up message с image payload,
   чтобы base64 не попал в observation и trace.
6. Цикл завершается финальным текстом или лимитом `max_turns`.

## На что обратить внимание

`AGENTS.md` сейчас добавляется в system prompt простой строковой склейкой. Это
хорошо для учебной ветки, но быстро становится тесно: prompt, история,
изображения и дополнительные источники контекста начинают конкурировать за
место. Именно эту проблему решает ветка `03-Context-Layer`.
