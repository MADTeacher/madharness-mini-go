# madharness-mini-go

> Учебная ветка: `04-Agents-Skills`
>
> Тема главы: project-local Agent Skills как управляемый способ добавлять
> агенту workflow-инструкции и ресурсы.
>
> В этой точке Go harness умеет искать `SKILL.md`, показывать каталог навыков
> модели, активировать skill через `activate_skill` и добавлять инструкции
> навыка в durable context.
>
> Лабораторные работы: [LABS.md](LABS.md)
> Предыдущая ветка: `03-Context-Layer`
> Следующая ветка: `05-mcp`

`madharness-mini-go` — учебный минималистичный harness для работы кодирующего
ИИ-агента с локальным программным продуктом. Он даёт модели понятный цикл:
получить задачу, увидеть контекст проекта, выбрать подходящий skill, вызвать
инструменты и записать ход выполнения в trace.

Проект написан для Go 1.25 и использует только стандартную библиотеку. Внутри
используется OpenAI-совместимый API `/chat/completions`, поэтому можно
подключить OpenRouter, KodikRouter, локальный совместимый сервер или другой
сервис с тем же форматом API.

## Что есть в этой ветке

- команды `init`, `ask`, `run`, `trace` и `skills`;
- проектные инструкции `AGENTS.md`;
- слой контекста с бюджетом и `context_report`;
- инструмент `read_image` для моделей с vision input;
- базовые инструменты workspace: `list_files`, `read_file`, `write_file`,
  `search_code`, `apply_patch`, `run_shell`;
- discovery project-local skills в `.madharness_mini/skills` и `.agents/skills`;
- явная активация через `@skill:name`, `@skill/name`, `$name` и похожие фразы;
- auto-activation через catalog и инструмент `activate_skill`;
- CLI-диагностика `skills list`, `skills show`, `skills validate`.

В этой ветке ещё нет MCP, субагентов и hooks. Здесь фокус только на skills как
локальном расширении контекста и workflow-памяти агента.

## Быстрый запуск

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

## Минимальный skill

Создайте файл `.madharness_mini/skills/docs-writer/SKILL.md`:

```md
---
name: docs-writer
description: Помогает обновлять README и учебную документацию проекта.
---

Перед правкой документации прочитай README, docs/README.md и связанные файлы.
Сохраняй короткий учебный стиль и не добавляй возможности, которых нет в коде.
```

Проверьте, что harness видит skill:

```bash
go run ./cmd/madharness-mini skills list
```

Запустите задачу с явным skill:

```bash
go run ./cmd/madharness-mini run "@skill:docs-writer обнови README"
```

Если skill не выбран явно, в `run` модель увидит компактный catalog и сможет
сама вызвать `activate_skill`.

## Документация ветки

- [Возможности ветки](docs/capabilities.md)
- [Структура кода](docs/code-overview.md)
- [Слой контекста](docs/context-layer.md)
- [Agent Skills](docs/agent-skills.md)
- [Инструмент apply_patch](docs/apply-patch.md)

## Разработка самого проекта

Если вы меняете код `madharness-mini-go`, запускайте проверки из корня
репозитория:

```bash
go test ./...
```

Быстрая ручная проверка CLI:

```bash
go run ./cmd/madharness-mini skills validate
```

## Что дальше

Следующая ветка `05-mcp` добавляет подключение внешних инструментов через
минимальный stdio MCP-клиент.

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
