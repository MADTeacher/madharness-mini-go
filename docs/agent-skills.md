# Agent Skills в madharness-mini-go

Agent Skill в `madharness-mini-go` — это project-local папка с инструкцией
`SKILL.md` и дополнительными файлами, которые помогают агенту выполнить
конкретный тип задач: писать документацию, проверять проект, готовить
диаграммы, редактировать учебный текст и т.п.

Skill не является плагином и не получает отдельного канала исполнения. Он
добавляет агенту контекст и, при необходимости, указывает на bundled resources:
`scripts`, `references`, `assets` и другие файлы внутри папки навыка. Все
реальные действия всё равно проходят через обычные инструменты harness и общую
политику безопасности.

## Зачем нужны skills

Обычные проектные инструкции `AGENTS.md` подходят для устойчивых правил всего
проекта: стиль кода, тестовая команда, ограничения безопасности. Agent Skills
решают другую задачу: они описывают специализированный workflow, который нужен
не всегда.

Главный принцип — прогрессивное раскрытие. Модель сначала видит только каталог
навыков, а полный `SKILL.md` попадает в контекст только после активации
конкретного навыка.

## Где лежат skills

`madharness-mini-go` ищет skills только внутри `workspace_root`, без отдельной
настройки в `config.json`.

Поддерживаются два каталога:

```text
.madharness-mini/skills
.agents/skills
```

Каждая прямая подпапка с валидным `SKILL.md` считается кандидатом. Если одно и
то же `name` найдено в обоих каталогах, версия из `.madharness-mini/skills`
перекрывает версию из `.agents/skills`.

## Формат SKILL.md

`SKILL.md` состоит из YAML-subset frontmatter и markdown-инструкций.

```markdown
---
name: docs-writer
description: Используй этот навык для задач про README, docs и описание API.
---

# Docs Writer

Пиши коротко, проверяемо и со ссылками на конкретные файлы.
```

Обязательные поля:

| Поле | Смысл |
| --- | --- |
| `name` | Имя навыка. Используется в каталоге, CLI и `activate_skill`. |
| `description` | Короткое описание, по которому модель решает, нужен ли навык. |

Дополнительные поля:

| Поле | Смысл |
| --- | --- |
| `license` | Показывается в `skills show` и activation wrapper. |
| `compatibility` | Попадает в activation wrapper, чтобы модель видела требования среды. |
| `metadata` | Простая карта `ключ: значение` для диагностики и описания навыка. |
| `allowed-tools` | Experimental-подсказка для модели. Не отменяет глобальную политику безопасности. |

Проект не добавляет YAML-библиотеку: parser поддерживает scalar-поля,
одноуровневый `metadata:` и простые block scalars вроде `>-`.

## Как работает run

Skills подключаются только в режиме `run`. Режим `ask` остаётся коротким
одиночным запросом: он не загружает skill catalog, не добавляет
`activate_skill` и не разбирает skill-маркеры.

Поток `run`:

1. `Config` определяет `workspace_root`.
2. Loader сканирует `.madharness-mini/skills` и `.agents/skills`.
3. Frontmatter каждого `SKILL.md` превращается в индекс skills.
4. В трассу пишется `skills_discovered`.
5. Если пользователь явно указал skill, он активируется до первого обращения к модели.
6. Если явного выбора нет, модель получает compact catalog.
7. Когда модель решает, что skill нужен, она вызывает `activate_skill`.
8. Harness добавляет полный workflow skill в durable context-фрагмент.
9. Модель продолжает работу уже с активными инструкциями.

## Каталог и активация

Catalog — это короткий системный фрагмент с `name`, `description` и путём к
`SKILL.md` внутри workspace. Полное тело `SKILL.md` на этом этапе не
раскрывается.

Явный выбор пользователя поддерживает формы:

- `@skill:docs-writer`;
- `@skill/docs-writer`;
- `$docs-writer`;
- фразы вида `используй навык docs-writer` или `use skill docs-writer`.

При явном выборе auto-selection отключается: catalog не добавляется, а
`activate_skill` не выдаётся модели.

Активированный skill добавляется как закреплённый `agentcontext.Fragment` с id:

```text
skill:<name>
```

Повторная активация того же skill не дублирует инструкции. Runtime возвращает
observation `skill already active`.

## Bundled resources

Файлы внутри skill root считаются bundled resources. Обычно это:

```text
references/
scripts/
assets/
```

При активации harness перечисляет ресурсы:

- относительный путь внутри skill root;
- workspace-relative путь;
- тип по первой папке (`references`, `scripts`, `assets` или другое);
- размер файла в байтах.

Содержимое ресурсов не читается автоматически. Если workflow говорит прочитать
справочник, модель должна вызвать обычный `read_file`. Если workflow говорит
запустить documented script, модель использует `run_shell` с `cwd`, равным skill
root.

## Безопасность

Skills не расширяют границы безопасности harness.

Основные правила:

- skill-каталоги должны находиться внутри `workspace_root`;
- `SKILL.md` читается как UTF-8 текст;
- symlink escape из skill root не считается bundled resource;
- scripts не запускаются сами по себе;
- resources читаются только обычными файловыми инструментами;
- `allowed-tools` не даёт разрешений сверх глобальной политики;
- `run_shell` по-прежнему проверяет команду через `Policy.ShellAllowed()`;
- файловые пути по-прежнему проверяются через `Policy.SafePath()`.

## CLI-команды

Посмотреть найденные skills:

```bash
go run ./cmd/madharness-mini skills list
```

Показать полный skill:

```bash
go run ./cmd/madharness-mini skills show docs-writer
```

Проверить диагностику discovery:

```bash
go run ./cmd/madharness-mini skills validate
```

## Trace

Каждый `run` пишет JSONL-трассу в отдельную директорию запуска:

```text
.madharness-mini/traces/<trace-id>/<trace-id>.jsonl
```

Команда `trace` также читает старые плоские файлы
`.madharness-mini/traces/<trace-id>.jsonl`.

Для skills есть отдельные события:

| Событие | Что означает |
| --- | --- |
| `skills_discovered` | Harness просканировал skill-каталоги и записал имена плюс диагностику. |
| `skills_explicit_selection` | Пользователь явно указал skill-маркеры в задаче. |
| `skills_auto_selection_disabled` | Auto-selection отключён из-за явного выбора. |
| `skill_activated` | Skill стал active context-фрагментом. |
| `skill_resource_used` | Инструмент обратился к файлу или cwd внутри активного skill root. |

Полный текст активированного `SKILL.md` не дублируется в trace. В
`context_report` видны только id, source и размер фрагмента.
