# MCP в madharness-mini-go

`madharness-mini-go` умеет подключать внешние stdio MCP-серверы в режиме `run`.
Для модели такие серверы выглядят как обычные tools харнесса: они попадают в
общий список схем, вызываются через `tool_calls` и возвращают привычные
observations с полями `ok`, `tool`, `summary` и дополнительными данными.

Реализация намеренно минимальная: только stdio transport, только MCP tools и
только стандартная библиотека Go. Основной `config.json` не меняется; MCP
включается отдельным файлом `.madharness-mini/mcp.json`.

## Быстрый пример

Создайте в проекте файл:

```text
.madharness-mini/mcp.json
```

Пример с reference MCP-сервером:

```json
{
  "servers": {
    "time": {
      "enabled": true,
      "command": "uvx",
      "args": ["mcp-server-time", "--local-timezone=Europe/Moscow"],
      "cwd": ".",
      "timeout_seconds": 90
    }
  }
}
```

После этого можно запустить:

```bash
go run ./cmd/madharness-mini run "Через MCP узнай текущее время"
```

Если файл `mcp.json` отсутствует, MCP полностью выключен и старые проекты
работают с прежним набором инструментов.

## Формат mcp.json

Корневое поле `servers` содержит объект, где ключ — локальное имя MCP-сервера.
Имя должно состоять из ASCII-букв, цифр, `_` или `-`.

| Поле | Обязательность | Смысл |
| --- | --- | --- |
| `enabled` | Да | Сервер запускается только при точном значении `true`. |
| `command` | Да | Исполняемая команда, например `npx`, `uvx`, `python3`. |
| `args` | Нет | Список строковых аргументов команды. По умолчанию пустой список. |
| `cwd` | Нет | Рабочий каталог сервера внутри `workspace_root`. По умолчанию `"."`. |
| `env` | Нет | Явные переменные окружения для сервера. Значения должны быть строками. |
| `timeout_seconds` | Нет | Таймаут одного MCP-запроса. По умолчанию `20`. |

Команда и аргументы не склеиваются в shell-строку. Harness запускает процесс как
`[command, *args]`, поэтому shell-операторы, пайпы и подстановки тут не работают.

## Как проходит запуск

В обычном `agent.Run()` добавляется `mcp.ToolProvider`. Если `mcp.json` нет, он
ничего не регистрирует. В режиме `--orchestrate-required` provider MCP не
добавляется: этот флаг сам по себе включает только обязательную оркестрацию и
не делает MCP tools доступными parent-run. Если файл есть и режим запуска
подключает MCP, поток такой:

1. `internal/mcp` читает `.madharness-mini/mcp.json`.
2. Для каждого `enabled: true` сервера проверяются `command`, `args`, `cwd`,
   `env` и `timeout_seconds`.
3. `cwd` проходит через общую `Policy.SafePath()`, поэтому сервер не может быть
   запущен из каталога вне workspace.
4. `StdioClient` запускает subprocess без shell.
5. Для stdout и stderr создаются отдельные reader goroutines.
6. Клиент отправляет `initialize` с протоколом `2025-11-25`.
7. Сервер должен вернуть тот же `protocolVersion`; иначе запуск считается
   ошибочным.
8. Клиент отправляет `notifications/initialized`.
9. Клиент вызывает `tools/list`.
10. Каждый MCP tool превращается в локальный `tools.Spec`.
11. `tools.Registry` добавляет эти specs рядом со встроенными tools.
12. Модель видит MCP tools в обычном поле `tools` OpenAI-совместимого запроса.

Если сервер не стартовал, вернул невалидный JSON, не ответил до таймаута или
прислал некорректный `tools/list`, запуск `run` завершается понятной ошибкой.
При ошибке регистрации уже запущенные provider-ы закрываются через
`tools.Registry.Close()`.

`initialize`, `notifications/initialized` и `tools/list` выполняются
последовательно во время старта. После регистрации tools соседние MCP-вызовы в
одном assistant-turn-е могут выполняться параллельно, если
`max_parallel_tool_calls` больше `1`. Stdio-клиент использует JSON-RPC
multiplexer: каждый request получает защищённый id, stdout-диспетчер доставляет
response в канал ожидающего запроса, а timeout или закрытие transport-а
освобождает pending-запросы без смешивания observations.

## Имена инструментов

MCP tool получает имя:

```text
mcp__<server_name>__<tool_name>
```

Например:

| MCP-сервер | Исходный tool | Имя для модели |
| --- | --- | --- |
| `time` | `get_current_time` | `mcp__time__get_current_time` |
| `fetch` | `fetch` | `mcp__fetch__fetch` |
| `playwright` | `browser_navigate` | `mcp__playwright__browser_navigate` |

Символы в имени tool, несовместимые с OpenAI function name, заменяются на `_`.
Итоговое имя ограничено 64 символами. Оригинальное MCP-имя сохраняется внутри
handler и используется при настоящем `tools/call`.

## Как MCP-ответ становится observation

`internal/mcp/results.go` приводит `tools/call` result к формату, который уже
понимает агентский цикл.

| MCP content | Что получает модель |
| --- | --- |
| `text` | Строки объединяются в `content` и обрезаются общим лимитом. |
| `structuredContent` | Попадает в поле `data`. |
| `image` и `audio` | Передаются только метаданные: тип, MIME type и размер base64-строки. |
| `resource` и `resource_link` | Передаются короткие метаданные ресурса: `uri`, `name`, MIME type, description. |
| Неизвестный content type | Попадает в `diagnostics`. |

Если MCP-сервер вернул `isError: true`, observation будет ошибочным
`ok: false`, даже если JSON-RPC ответ технически успешен. Если ломается сам
transport или JSON-RPC, handler возвращает обычный `fail()` observation.

## Окружение и безопасность

MCP-сервер — это внешний процесс, поэтому его запуск должен быть явным и
локальным для проекта.

Harness наследует только небольшой безопасный набор системных переменных:
`PATH`, `HOME`, `USER`, `TMPDIR`, `TEMP`, `TMP`, `LANG`, `LC_ALL`, `ComSpec`,
`SystemRoot`, `WINDIR`. Затем добавляется явный `env` из `mcp.json`.

Переменные `MADHARNESS_MINI_*` и ключ LLM API не передаются MCP-серверам
автоматически. Если конкретному серверу нужен токен, его нужно указать явно в
`env`, понимая, что это доверенный локальный процесс.

## Trace и закрытие процессов

MCP пишет отдельные события в JSONL-трассу:

| Событие | Когда пишется |
| --- | --- |
| `mcp_server_started` | Сервер прошёл `initialize` и `tools/list`. |
| `mcp_server_error` | Сервер не смог стартовать или отдать tools. |
| `mcp_server_stopped` | Provider закрывает subprocess. |
| `tool_observation` | Модель вызвала MCP tool. |

Полный stdout/stderr MCP-сервера в трассу не пишется. Для ошибок stderr
добавляется только коротким фрагментом в текст исключения.
План выполнения tools пишет MCP batches в `tool_execution_plan` с kind `mcp`;
команда `trace` показывает их отдельным счётчиком parallel MCP batches.

`tools.Registry.Close()` вызывается в `agent.Run()` через `defer`. MCP provider
сначала закрывает stdin сервера, ждёт штатного завершения, затем пробует мягкий
сигнал и в последнюю очередь принудительное завершение процесса.

## Ограничения текущей версии

Поддерживаются:

- stdio transport;
- `initialize`;
- `notifications/initialized`;
- `tools/list`;
- `tools/call`;
- базовая обработка server-to-client requests через ответ `Method not found`.

Не поддерживаются:

- Streamable HTTP и SSE transport;
- MCP resources как отдельная возможность клиента;
- MCP prompts;
- pagination для `tools/list`;
- sampling;
- roots;
- elicitation.
