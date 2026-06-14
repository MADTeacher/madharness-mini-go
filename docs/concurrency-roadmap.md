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

`max_parallel_subagents` управляет конкурентным запуском соседних
`delegate_task` внутри одного parent turn-а. Это не отдельный лимит model calls:
каждый субагент работает как дочерняя agent session со своей trace, context и
registry, а число параллельных child sessions ограничивает число параллельных
subagent model loops.

```bash
go run ./cmd/madharness-mini run --max-parallel-subagents 2 --orchestrate "..."
```

Параллельно могут выполняться соседние read-only tools:

- `list_files`;
- `read_file`;
- `read_image`;
- `search_code`.

Соседние MCP tools образуют отдельный parallel batch и используют тот же
`max_parallel_tool_calls`. Внутри одного stdio-сервера ответы маршрутизируются
через JSON-RPC multiplexer, поэтому поздний response одного запроса не может
попасть в другой tool call.

Остальные tools остаются барьерами и выполняются по одному:

- `write_file`;
- `apply_patch`;
- `run_shell`;
- `start_shell`;
- `shell_status`;
- `stop_shell`;
- `activate_skill`;
- неизвестные tools без явного effect.

Соседние `delegate_task` образуют отдельный parallel batch. Если один
субагент просит ввод пользователя или завершается ошибкой, parent сначала
дожидается уже запущенных sibling-субагентов, а затем применяет observations в
исходном порядке tool calls.

`before_tool_call` вызывается синхронно и в порядке ответа модели до запуска
handler-ов. После выполнения handlers harness применяет observations, hidden
effects, trace events и `RecordToolResult()` строго в исходном порядке
`tool_calls`, даже если read-only или MCP handlers завершились в другом
порядке.

Перед выполнением tools harness пишет `tool_execution_plan` в trace: лимиты,
группы выполнения, тип группы, имена tools и классы effects. Команда `trace`
показывает краткую строку `concurrency` с max tool calls, max subagents,
количеством read/MCP/delegate batches и barrier-групп.

### Agent sessions

Root-agent и каждый субагент выполняются как отдельные agent sessions. У
сессии свои context, registry, event bus и trace; общими остаются config,
model client, hook providers, approval prompt guard и workspace scheduler.
История parent-сессии не копируется в child-сессию: субагент получает только
свою `task` и явный `context` из аргументов `delegate_task`.

### Managed shell processes

Для долгих dev-сценариев доступны отдельные shell tools:

- `start_shell` запускает разрешённую команду как managed process текущего
  `run` и возвращает `process_id`;
- `shell_status` показывает состояние, readiness, exit code и buffered
  stdout/stderr одного процесса или всего списка;
- `stop_shell` мягко завершает процесс и при необходимости убивает его.

Процессы живут только в памяти текущего `run`. Root-agent и субагенты видят
один общий process manager, поэтому parent может поднять backend/frontend, а
субагент или следующий turn - проверить их через MCP tools. При завершении
`run` manager останавливает оставшиеся процессы.

`start_shell` использует ту же shell policy, approval flow и проверку
workspace-relative `cwd`, что и `run_shell`. Опциональный `ready_pattern`
матчится по stdout/stderr и позволяет дождаться сообщения вроде
`Listening on ...` без отдельного HTTP-polling. Живой managed process не держит
global workspace lock: lock берётся только на start/stop, чтобы агент мог
читать файлы, запускать второй процесс и вызывать MCP tools, пока серверы
работают.

Trace получает события `process_started`, `process_output`,
`process_stopped` и `process_failed`.

### Workspace scheduler

File/shell tools всех сессий проходят через общий workspace scheduler:

- `list_files` и `read_file` берут path read lock;
- `write_file` берёт path write lock;
- `apply_patch` сначала вычисляет touched paths, затем берёт write locks на
  весь набор и применяет patch;
- `run_shell` берёт global exclusive lock на время subprocess.
- `start_shell` и `stop_shell` берут global exclusive lock только на операцию
  запуска или остановки managed process.

Locks не удерживаются во время model calls, lifecycle hooks preflight и
approval prompt.

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

### Session finalizer

`session_end` и `session_error` пока пишутся из нескольких веток agent loop.
Для более конкурентного runtime нужен session finalizer или state machine с
once-семантикой, cancellation и обязательным flush дочерних задач.

### Trace spans

Trace v1 получил последовательный `seq`, но будущая диагностика должна добавить
`event_id`, `span_id`, `parent_span_id`, явный flush contract и связи между
parent trace, child trace и конкретным tool call.
