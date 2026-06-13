# madharness-mini-go

`madharness-mini-go` — учебный минималистичный harness для курса, книги и
лабораторных работ по harness-инженерии кодирующих ИИ-агентов.

Репозиторий устроен как учебный маршрут: каждая ветка фиксирует отдельную
ступень развития harness, а внутри ветки лежат актуальные для этой ступени
`README.md`, `LABS.md` и `docs/`.

Ветка `main` показывает полную версию Go-порта с проектными инструкциями,
слоем контекста, Agent Skills, MCP, субагентами и hooks.

Проект написан для Go 1.25 и использует только стандартную библиотеку. Внутри
используется OpenAI-совместимый API `/chat/completions`, поэтому можно
подключить OpenRouter, KodikRouter, локальный совместимый сервер или другой
сервис с тем же форматом API.

## Учебный маршрут

| Ветка | Тема | Главный вопрос |
| --- | --- | --- |
| [`01-minimalistic-harness`](https://github.com/MADTeacher/madharness-mini-go/tree/01-minimalistic-harness) | Минимальный harness | Как устроить базовый цикл: модель, инструменты, трасса? |
| [`02-AGENTS-md`](https://github.com/MADTeacher/madharness-mini-go/tree/02-AGENTS-md) | Проектные инструкции и изображения | Как добавить локальные правила проекта и vision input? |
| [`03-Context-Layer`](https://github.com/MADTeacher/madharness-mini-go/tree/03-Context-Layer) | Слой контекста | Что именно модель видит перед каждым вызовом? |
| [`04-Agents-Skills`](https://github.com/MADTeacher/madharness-mini-go/tree/04-Agents-Skills) | Agent Skills | Как подключать рабочие инструкции без изменения ядра? |
| [`05-mcp`](https://github.com/MADTeacher/madharness-mini-go/tree/05-mcp) | MCP-инструменты | Как превратить внешний stdio MCP-сервер в обычные инструменты модели? |
| [`06-subagents`](https://github.com/MADTeacher/madharness-mini-go/tree/06-subagents) | Субагенты | Как делегировать задачи ролям с отдельными инструментами и трассами? |
| [`07-hooks`](https://github.com/MADTeacher/madharness-mini-go/tree/07-hooks) | Hooks | Как добавить проектный аудит и блокировку действий? |

Подробная карта курса: [COURSE.md](COURSE.md).

В каждой учебной ветке:

- `README.md` объясняет, где вы находитесь и что умеет эта версия;
- `LABS.md` содержит задачи трёх уровней без оценок времени;
- `docs/README.md` ведёт к актуальным документам этой ветки.

## Быстрый старт финальной версии

Перейдите в корень проекта:

```bash
cd /path/to/madharness-mini-go
```

Создайте локальную настройку:

```bash
go run ./cmd/madharness-mini init \
  --base-url https://openrouter.ai/api/v1 \
  --model deepseek/deepseek-v4-flash \
  --api-key "ключ-доступа-openrouter"
```

Если ключ уже лежит в `.env`, достаточно:

```bash
go run ./cmd/madharness-mini init --no-prompt
```

Задайте вопрос без инструментов:

```bash
go run ./cmd/madharness-mini ask "Объясни, что делает этот проект"
```

Запустите агентский режим:

```bash
go run ./cmd/madharness-mini run "Найди команду для запуска тестов и объясни, что она проверяет"
```

Посмотрите трассу:

```bash
go run ./cmd/madharness-mini trace <trace-id>
```

## Финальная документация

- [Возможности полной версии](docs/capabilities.md)
- [Структура кода](docs/code-overview.md)
- [Слой контекста](docs/context-layer.md)
- [Agent Skills](docs/agent-skills.md)
- [MCP](docs/mcp.md)
- [Субагенты](docs/subagents.md)
- [Hooks](docs/hooks.md)
- [Инструмент apply_patch](docs/apply-patch.md)

## Разработка самого проекта

Если вы меняете код `madharness-mini-go`, запускайте проверки из корня
репозитория:

```bash
go test ./...
```

Быстрая ручная проверка CLI:

```bash
go run ./cmd/madharness-mini ask "Объясни, что делает этот проект"
go run ./cmd/madharness-mini run "Найди команду для запуска тестов и объясни, что она проверяет"
```

## Лицензирование

Проект использует раздельную лицензионную модель:

- код распространяется по PolyForm Noncommercial License 1.0.0;
- учебные материалы распространяются по Creative Commons
  Attribution-NonCommercial-ShareAlike 4.0 International.

Некоммерческое самообучение, академическое преподавание и исследовательское
использование разрешены на условиях соответствующих лицензий. Коммерческое
использование кода или материалов требует предварительного письменного
разрешения правообладателя.

Полные тексты и русские версии: [LICENSE.md](LICENSE.md).
