# Конкурентность harness

Этот документ отделяет реализованное поведение v1 от следующих шагов. Ветка
остаётся учебной: конкурентность добавляется там, где она не ломает порядок
сообщений модели, event bus, hooks и trace.

## V1 implemented behavior

`max_parallel_tool_calls` управляет конкурентным выполнением tool calls внутри
одного assistant-turn. По умолчанию значение равно `1`, поэтому старые запуски
ведут себя последовательно.

Разово для запуска лимит можно переопределить CLI-флагом:

```bash
go run ./cmd/madharness-mini run --max-parallel-tool-calls 2 "..."
```

Параллельно могут выполняться только соседние read-only tools:

- `list_files`;
- `read_file`;
- `read_image`;
- `search_code`.

Остальные tools остаются барьерами и выполняются по одному:

- `write_file`;
- `apply_patch`;
- `run_shell`;
- `activate_skill`;
- `delegate_task`;
- MCP tools;
- неизвестные tools без явного effect.

`before_tool_call` вызывается синхронно и в порядке ответа модели до запуска
handler-ов. После выполнения handlers harness применяет observations, hidden
effects, trace events и `RecordToolResult()` строго в исходном порядке
`tool_calls`, даже если read-only handlers завершились в другом порядке.

Перед выполнением tools harness пишет `tool_execution_plan` в trace: лимит,
группы выполнения, имена tools и классы effects. Команда `trace` показывает
краткую строку `concurrency` с max parallel, количеством read batches и
barrier-групп.

### Внутренняя event bus

Lifecycle-точки `ask`, `run` и `subagent` публикуются через `internal/events`.
Trace и hooks получают одно событие как подписчики: trace сохраняет прежние
JSONL event names, а hooks получают прежний JSON-контракт `version: 1`.

### Observe/enforce hooks

Hooks разделены на `enforce` и `observe`. Старый `hooks.json` без `mode`
остаётся `enforce`. `enforce` hooks остаются синхронными, но блокировать ход
могут только `before_tool_call` и `approval_request`. `observe` hooks
выполняются через очередь 32 элемента и никогда не блокируют действие; при
переполнении пишется `hook_failed`.

### Approval foundation

Эскалируемые policy-отказы проходят через `internal/approval`: trace и hooks
видят `approval_request`, затем `approval_decision`. По умолчанию
`approval_mode` равен `deny`; `--approval ask` включает CLI prompt, а `--yolo`
автоматически подтверждает эскалируемые отказы внутри workspace. Пути за
пределами workspace и невалидные команды не эскалируются.

## Future work

### Parallel subagents

`delegate_task` пока остаётся barrier. Для параллельных субагентов нужен общий
лимит model calls, корректная отмена, дочерние trace spans и понятный порядок
возврата результатов parent-агенту.

### Workspace scheduler

V1 сериализует все write/shell действия. Более сильная версия может ввести
global workspace scheduler: read locks, path-level write locks, global locks для
опасных tools и общий механизм для parent и subagents.

### Managed shell processes

Долгий shell-запуск пока не реализован и не должен маскироваться обычным
`run_shell`. Следующий шаг - отдельные tools `start_shell`, `shell_status` и
`stop_shell`: первый создаёт управляемый процесс и сразу возвращает process id,
второй читает buffered stdout/stderr и состояние, третий мягко завершает
процесс и при необходимости убивает его. Lifecycle-события должны быть
совместимы с текущей event bus: `process_started`, `process_output`,
`process_stopped` и `process_failed`.

### MCP multiplexer

MCP tools сейчас сериализуются на уровне одного stdio server. Полноценная
конкурентность требует JSON-RPC multiplexer: защищённые request id, pending map
`id -> response channel`, dispatcher stdout и закрытие transport-а без гонок.

### Session finalizer

`session_end` и `session_error` пока пишутся из нескольких веток agent loop.
Для более конкурентного runtime нужен session finalizer или state machine с
once-семантикой, cancellation и обязательным flush дочерних задач.

### Trace spans

Trace v1 получил последовательный `seq`, но будущая диагностика должна добавить
`event_id`, `span_id`, `parent_span_id`, явный flush contract и связи между
parent trace, child trace и конкретным tool call.
