# Субагенты

Субагенты позволяют ведущему агенту делегировать часть работы отдельной роли:
исследователю, планировщику, реализатору, ревьюеру или пользовательскому
исполнителю из проекта. Для пользователя это остаётся обычным режимом `run`, но
доступность `delegate_task` зависит от режима оркестрации.

Субагент запускается в том же процессе, но получает отдельный контекст, свой
системный prompt, свой список доступных tools и свой JSONL trace. История
ведущего агента не копируется целиком; в делегацию попадают только задача и
короткий `context`, который ведущий агент передал в `delegate_task`.

Внутри runtime главный агент и каждый субагент представлены отдельной
agent session. Сессии разделяют config, model client, hook providers, approval
prompt guard и workspace scheduler, но не разделяют историю контекста.

## Встроенные роли

Встроенные субагенты лежат в пакете:

```text
internal/subagents/prompts/subagents
```

| Роль | Профиль | Tools | Для чего нужна |
| --- | --- | --- | --- |
| `researcher` | `read-only` | `list_files`, `read_file`, `search_code` | Найти факты в проекте перед планированием или правкой. |
| `planner` | `writable` | `list_files`, `read_file`, `search_code`, `apply_patch`, `write_file`, `ask_user` | Вести markdown-файл плана, разобрать неоднозначную задачу и при необходимости спросить пользователя. |
| `implementer` | `writable` | `list_files`, `read_file`, `search_code`, `apply_patch`, `write_file`, `run_shell` | Внести кодовые изменения и запустить безопасную проверку. |
| `reviewer` | `read-only` | `list_files`, `read_file`, `search_code` | Проверить изменение или план на баги, регрессии и тестовые пробелы. |

## Пользовательские субагенты

Проект может добавить собственных субагентов без изменения кода
`madharness-mini-go`. Для этого создайте файл:

```text
.madharness-mini/subagents/<name>.md
```

Минимальный пример:

```yaml
---
name: test-writer
description: Пишет минимальные Go-тесты для изменённого кода.
profile: writable
tools: ["list_files", "read_file", "search_code", "apply_patch", "write_file", "run_shell", "ask_user"]
max_turns: 10
context_max_tokens: 30000
---

Ты субагент test-writer.

Твоя задача - писать маленькие Go-тесты рядом с существующими тестами.
Сначала найди похожие тесты, затем добавь минимальную проверку.
Финальный ответ должен перечислить изменённые файлы и команду проверки.
```

Если пользовательский файл использует имя встроенной роли, он перекрывает её
только при явном поле:

```yaml
override: true
```

Без `override: true` встроенная роль остаётся активной, а
`subagents validate` покажет warning.

## Поля frontmatter

| Поле | Обязательное | Смысл |
| --- | --- | --- |
| `name` | да | Имя субагента, которое передаётся в `delegate_task`. |
| `description` | да | Короткое описание для ведущего агента и CLI. |
| `profile` | да | `read-only` или `writable`. |
| `tools` | да | JSON-style список строк, например `["read_file", "search_code"]`. |
| `max_turns` | нет | Лимит ходов конкретного субагента. Если не задан, берётся `subagent_max_turns`. |
| `context_max_tokens` | нет | Контекстный бюджет конкретного субагента. Если не задан, берётся `subagent_context_max_tokens`. |
| `override` | нет | Разрешает project-local файлу перекрыть встроенную роль с тем же именем. |
| `metadata` | нет | Одноуровневый словарь строк для заметок проекта. |

Важно: поле `tools` пишется именно как список строк в JSON-стиле:

```yaml
tools: ["list_files", "read_file", "search_code"]
```

Формат вроде `tools: list_files read_file search_code` считается ошибкой.
Список `tools` проверяется против child registry: субагент может получить
только встроенные tools и `ask_user`. `delegate_task`, `activate_skill`,
MCP-tools вида `mcp__...` и другие parent/custom tools намеренно не выдаются
субагентам; `subagents validate` отклонит такие имена.

## Profiles и tools

`profile` описывает общий уровень доверия роли, но сам по себе не выдаёт
инструменты. Фактические полномочия всегда задаёт список `tools`.

`read-only` обычно получает только чтение и поиск: `list_files`, `read_file`,
`search_code`. Такой субагент подходит для исследования и ревью.

`writable` может получить `apply_patch`, `write_file` и, если это нужно роли,
`run_shell`. Для встроенного `planner` действует дополнительный write scope: он
может создавать и менять только markdown-файлы (`.md`) с составляемым планом.

Ведущий агент может попросить writable-субагента запуститься как `read-only`.
Такой downgrade убирает потенциально пишущие tools: `apply_patch`,
`write_file` и `run_shell`. Обратный upgrade запрещён: `read-only` роль нельзя
сделать writable через аргументы `delegate_task`.

## `ask_user`

`ask_user` доступен только внутри субагента, если имя инструмента указано в
`tools`. Он не читает stdin и не открывает интерактивный prompt. Вместо этого
субагент завершает текущую делегацию со статусом `needs_user_input`, а основной
`run` сразу завершается прямым вопросом пользователю.

Пример ожидаемого поведения:

```text
planner -> ask_user(question="Какой API оставить публичным?", options=["A", "B"])
delegate_task -> status: needs_user_input
madharness-mini run -> печатает вопрос пользователю и завершает текущий запуск
```

## Делегация

В режиме `run` ведущий агент видит tool `delegate_task`, если текущий режим
оркестрации это разрешает.

Аргументы:

| Поле | Смысл |
| --- | --- |
| `subagent` | Имя встроенной или project-local роли. |
| `task` | Маленькая самодостаточная задача для субагента. |
| `context` | Короткий родительский контекст: ограничения, решения, файлы, которые уже известны. |
| `profile` | Необязательный запрос `read-only` или `writable`; используется только для безопасного downgrade. |

Observation `delegate_task` содержит:

| Поле | Смысл |
| --- | --- |
| `status` | `done`, `needs_user_input` или ошибка. |
| `answer` | Финальный ответ субагента, если он завершился успешно. |
| `question` | Вопрос для пользователя, если статус `needs_user_input`. |
| `changed_files` | Пути, которые субагент изменил через `write_file` или `apply_patch`. |
| `trace_summary` | Краткая сводка дочерней трассы. |
| `subagent_trace_id` | Идентификатор локального trace субагента. |
| `subagent_trace_path` | Путь к локальному trace относительно рабочей папки, если это возможно. |

Соседние `delegate_task` внутри одного ответа модели могут выполняться
параллельно, если `max_parallel_subagents` больше `1`. Parent-agent всё равно
получает observations в исходном порядке tool calls. Если один субагент просит
ввод пользователя, harness дожидается уже запущенных sibling-субагентов и затем
завершает parent run вопросом.

## Трассы

Родительский запуск пишет события:

```text
subagents_discovered
subagent_started
subagent_finished
subagent_failed
user_input_requested
```

Полный ход субагента пишется в отдельный файл:

```text
.madharness-mini/traces/<parent-id>/<parent-id>--subagent-<name>-<suffix>.jsonl
```

Родительская трасса лежит рядом:
`.madharness-mini/traces/<parent-id>/<parent-id>.jsonl`.

Команда `trace` показывает наличие subagent-событий:

```bash
go run ./cmd/madharness-mini trace <trace-id>
```

## CLI

Посмотреть доступные роли:

```bash
go run ./cmd/madharness-mini subagents list
```

Показать конкретную роль вместе с prompt:

```bash
go run ./cmd/madharness-mini subagents show planner
```

Проверить frontmatter, коллизии и ошибки формата:

```bash
go run ./cmd/madharness-mini subagents validate
```

## Настройки

Глобальные настройки находятся в `.madharness-mini/config.json`.

| Поле | Значение по умолчанию | Смысл |
| --- | --- | --- |
| `orchestration_enabled` | `true` | Устаревший общий выключатель; `false` отключает `delegate_task`, если `orchestration_mode` оставлен в `auto`. |
| `orchestration_mode` | `auto` | Режим доступности оркестрации: `off`, `requested`, `auto`, `required`. |
| `max_parallel_subagents` | `1` | Сколько соседних `delegate_task` parent-session может запускать одновременно. |
| `subagent_max_turns` | `10` | Лимит ходов субагента без собственного `max_turns`. |
| `subagent_context_max_tokens` | `30000` | Контекстный бюджет субагента без собственного `context_max_tokens`. |

CLI-флаги одного запуска:

```bash
go run ./cmd/madharness-mini run --no-orchestrate "..."
go run ./cmd/madharness-mini run --orchestrate "..."
go run ./cmd/madharness-mini run --orchestration requested "..."
go run ./cmd/madharness-mini run --orchestrate-required "..."
go run ./cmd/madharness-mini run --max-parallel-subagents 2 --orchestrate "..."
```

`--orchestrate-required` сужает parent-run до обязательной оркестрации через
`delegate_task`. Он не добавляет `mcp.ToolProvider`, поэтому MCP tools не
появляются только из-за этого флага.

То же можно задать в `.env`:

```text
MADHARNESS_MINI_ORCHESTRATION_MODE=requested
MADHARNESS_MINI_MAX_PARALLEL_SUBAGENTS=2
```

## Безопасность

Субагенты используют тот же `Policy`, что и основной агент. Файловые tools не
могут выйти за `workspace_root`; такие отказы не эскалируются как
`protected_paths`. Пути из `protected_paths` запрещены по умолчанию, но
model-invoked файловое действие внутри workspace может быть разрешено через
approval flow или YOLO-режим.
Shell-команды проходят обычную проверку `run_shell`: запрещены управляющие
операторы shell и явно рискованные команды.

Файловые и shell tools всех сессий проходят через общий workspace scheduler:
чтения получают read lock, записи и `apply_patch` - write lock на затронутые
пути, а `run_shell` - global exclusive lock на время subprocess.

Project-local субагенты читаются только из `.madharness-mini/subagents/*.md`.
Они не получают особых прав на файловую систему: даже writable-роль может
писать только через обычные tools и только в рамках политики проекта.
