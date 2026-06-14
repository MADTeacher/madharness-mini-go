# Дорожная карта рефакторинга

Документ фиксирует результаты полного ревью и помогает разнести исправления по
независимым рабочим пакетам. Критичные пакеты уже выполнены, а оставшийся
backlog отделяет follow-up задачи и открытые design decisions.

## Выполнено

Эти пакеты уже были распределены по независимым субагентам и интегрированы.

| Пакет | Область | Результат |
| --- | --- | --- |
| Path/files safety | `internal/policy`, `internal/tools/filetools`, `internal/tools/searchtool`, `internal/tools/imagetool`, `internal/tools/patchtool`, `internal/instructions` | Закрыты symlink escape, protected path bypass и чтение больших или специальных файлов до лимитов. |
| Shell/process lifecycle | `internal/policy/shell*`, `internal/tools/shelltool`, `internal/processes` | Рискованные команды проверяются по argv, background cases закрыты, process tree завершается, readiness timeout считается ошибкой. |
| Subagents runtime | `internal/subagents`, `internal/agent`, `internal/agent/turnexec` | Исправлены read-only downgrade, остановка после `ask_user`, `changed_files` только для успешных записей и диагностика неизвестных tools. |
| Agent Skills path | `internal/skills` | Loader приведён к документированному `.madharness-mini/skills`, override-поведение закреплено тестами. |
| Config/control-plane safety | `internal/config`, `internal/policy` tests | Env overrides не сохраняются через `init`; control-plane файлы защищены по умолчанию без поломки штатного init/trace. |
| Hooks redaction | `internal/hooks` | Очевидные секреты маскируются не только по ключам, но и внутри значений `command`, `content` и похожих payload-полей. |
| Documentation corrections | `docs/`, кроме этого файла | Документация уточняет `protected_paths`, `--orchestrate-required`, branch-neutral режим `run` и фактический охват `skill_resource_used`. |
| Apply patch E2E coverage | `internal/tools/builtin/builtin_test.go` | Добавлены end-to-end тесты `apply_patch` на delete, move, outside workspace denial и protected path denial. |
| MCP flaky timing test | `internal/agent/mcp_integration_test.go` | Проверка overlap больше не зависит от общего wall-clock threshold и использует handshake fake MCP calls. |
| Concurrency test handshakes | `internal/agent`, `internal/agent/turnexec`, `internal/workspace` tests | Негативные ожидания `time.After(100ms)` заменены на причинные guards/handshakes. |

## Отложенный backlog

Оставшиеся задачи требуют решений по желаемому поведению или удобнее делаются
после интеграции основных safety-исправлений.

### Test Coverage

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
