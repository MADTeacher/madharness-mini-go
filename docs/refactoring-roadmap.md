# Дорожная карта рефакторинга

Документ фиксирует результаты полного ревью и помогает разнести исправления по
независимым рабочим пакетам. Критичные пакеты уже можно выполнять параллельно,
а оставшиеся пункты стоит брать после интеграции первых правок.

## В работе

Эти направления можно править независимо: у них разные владельцы файлов и низкий
риск конфликтов при слиянии.

| Пакет | Область | Цель |
| --- | --- | --- |
| Path/files safety | `internal/policy`, `internal/tools/filetools`, `internal/tools/searchtool`, `internal/tools/imagetool`, `internal/tools/patchtool`, `internal/instructions` | Закрыть symlink escape, protected path bypass и чтение больших или специальных файлов до лимитов. |
| Shell/process lifecycle | `internal/policy/shell*`, `internal/tools/shelltool`, `internal/processes` | Проверять рискованные команды по argv, закрыть `&`/background cases, завершать process tree и считать readiness timeout ошибкой. |
| Subagents runtime | `internal/subagents`, `internal/agent`, `internal/agent/turnexec` | Починить read-only downgrade, остановку после `ask_user`, `changed_files` только для успешных записей и диагностику неизвестных tools. |
| Agent Skills path | `internal/skills` | Привести loader к документированному `.madharness-mini/skills` и закрепить override-поведение тестами. |
| Config/control-plane safety | `internal/config`, при необходимости `internal/policy` tests | Не сохранять временные env overrides через `init`; защитить control-plane файлы по умолчанию без поломки штатного init/trace. |
| Hooks redaction | `internal/hooks` | Маскировать очевидные секреты не только по ключам, но и внутри значений `command`, `content` и похожих payload-полей. |

## Отложенный backlog

Эти задачи лучше не смешивать с текущими пакетами: часть из них документальная,
часть требует решений по желаемому поведению, а часть удобнее делать после
интеграции основных safety-исправлений.

### Documentation Corrections

- Уточнить модель `protected_paths`: сейчас они запрещены по умолчанию, но могут
  быть разрешены через approval или YOLO; пути вне workspace не эскалируются.
- Уточнить поведение MCP в `--orchestrate-required`: required orchestration не
  добавляет `mcp.ToolProvider`, поэтому MCP tools не появляются в root run.
- Убрать устаревшие веточные формулировки вроде `05-mcp` и заменить их
  branch-neutral описанием текущего режима `run`.
- Сверить описание `skill_resource_used`: сейчас событие пишется для `read_file`
  и `run_shell` cwd, но не для `write_file` и `apply_patch`.

### Test Coverage

- Добавить end-to-end тесты `apply_patch` на delete, move, outside workspace
  denial и protected path denial по умолчанию.
- Сделать concurrency tests менее завязанными на отрицательное ожидание
  `time.After(100ms)`; заменить на детерминированные handshakes там, где это
  не усложнит учебный код.
- Починить flaky race-проверку `TestRunOverlapsMCPToolsWhenParallelEnabled`:
  проверять факт overlap через trace/handshake, а не общий wall-clock threshold.
- Добавить concurrent-run тест или запретить shared `*config.Config` там, где
  `RunWithOptions` временно меняет поля config для model payload.

### Follow-Up Design Decisions

- Решить, должны ли harness control-plane файлы (`AGENTS.md`,
  `.madharness-mini/hooks.json`, `.madharness-mini/mcp.json`,
  `.madharness-mini/subagents`) быть protected по умолчанию или только
  документированным проектным риском.
- Решить, нужна ли backward compatibility для старого
  `.madharness_mini/skills`, если loader переезжает на `.madharness-mini/skills`.
- Решить, должны ли project-local субагенты получать доступ к MCP/custom tools
  или validator должен явно запрещать всё, чего нет в child registry.

