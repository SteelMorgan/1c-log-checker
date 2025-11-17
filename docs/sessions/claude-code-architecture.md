# Архитектура Claude Code: Где передаются инструкции

## Обзор

Claude Code получает инструкции из нескольких источников, которые комбинируются в единый контекст перед каждым сообщением.

---

## 1. Системный промпт (System Prompt)

**Расположение:** Передаётся при инициализации каждой сессии, находится в начале контекста

**Содержит:**

### 1.1 Базовая идентификация
```
You are Claude Code, Anthropic's official CLI for Claude.
You are an interactive CLI tool that helps users with software engineering tasks.
Model: claude-sonnet-4-5-20250929
Knowledge cutoff: January 2025
```

### 1.2 Основные правила поведения
- **Tone and style** - краткость, конкретность, избегание эмодзи
- **Professional objectivity** - приоритет точности над валидацией мнений
- **Output format** - GitHub-flavored markdown, monospace font

### 1.3 Инструкции по инструментам
Детальные правила использования каждого инструмента:
- **Task** - когда запускать агентов, какие типы агентов доступны
- **Bash** - правила работы с командами, git workflow, создание PR
- **Read/Write/Edit** - работа с файлами
- **Glob/Grep** - поиск файлов и контента
- **TodoWrite** - управление задачами
- **Skill/SlashCommand** - вызов навыков и команд

### 1.4 Специализированные workflow
- **Git commits** - детальный протокол создания коммитов
- **Pull requests** - как создавать PR через gh
- **Code references** - формат `file_path:line_number`

### 1.5 Политики безопасности
```
IMPORTANT: Assist with authorized security testing, defensive security,
CTF challenges, and educational contexts. Refuse requests for destructive
techniques, DoS attacks, mass targeting...
```

### 1.6 Tool permissions (белый список)
```
You can use the following tools without requiring user approval:
- Bash(dir:*)
- Bash(findstr:*)
- Bash(powershell -Command:*)
- WebFetch(domain:code.claude.com)
- Bash(powershell -ExecutionPolicy Bypass -File "D:\...\setup_skills.ps1")
- Skill(skill-creator-v2)
- WebFetch(domain:infostart.ru)
- Bash(git add:*)
- Bash(git commit:*)
- etc.
```

---

## 2. Environment Context (`<env>`)

**Передаётся:** В каждой сессии как структурированные данные

```xml
<env>
Working directory: D:\My Projects\FrameWork 1C\1c-log-checker
Is directory a git repo: Yes
Platform: win32
OS Version:
Today's date: 2025-11-17
</env>
```

**Обновляется:** Единожды в начале сессии (не динамически)

---

## 3. Git Status

**Передаётся:** В начале сессии как отдельный блок

```
gitStatus: This is the git status at the start of the conversation.
Note that this status is a snapshot in time, and will not update
during the conversation.

Current branch: master
Main branch: (you will usually use this for PRs):

Status:
M docs/techlog/all_techlog_properties.md
M docs/testing/comparison-guide.md
M internal/mcp/tools.json
M "заметка.md"
?? scripts/test_techlog_workflow.ps1

Recent commits:
6b93e1d feat: extend techlog functionality...
b660ed9 Fix: Tech log handler scanning...
```

---

## 4. Skills (Навыки)

### 4.1 Где хранятся
```
~/.config/claude/skills/           # Системные skills
.claude/skills/                     # Проектные skills (user location)
```

### 4.2 Как загружаются

**Через инструмент Skill:**
```typescript
Skill(skill: "skill-name")
```

**Результат:** Промпт навыка **расширяется** в контекст
```
<command-message>The "skill-name" skill is loading</command-message>
[Содержимое skill промпта вставляется в контекст]
```

### 4.3 Доступные skills (видимые в системном промпте)

```xml
<available_skills>
  <skill>
    <name>1c-bsl</name>
    <description>
      Skill for generating 1C:Enterprise (BSL) code with mandatory
      validation through MCP tools to prevent hallucinations.
    </description>
    <location>user</location>
  </skill>

  <skill>
    <name>go</name>
    <description>
      Expert guide for Go development with Clean Architecture,
      microservices patterns, and OpenTelemetry observability.
    </description>
    <location>user</location>
  </skill>

  <skill>
    <name>mcp-builder</name>
    <description>
      Guide for creating high-quality MCP servers...
    </description>
    <location>user</location>
  </skill>

  <skill>
    <name>skill-creator-v2</name>
    <description>
      Guide for creating bulletproof skills with built-in enforcement.
    </description>
    <location>user</location>
  </skill>

  <!-- ... ещё 14 skills -->
</available_skills>
```

### 4.4 Структура skill файла

**Пример:** `.claude/skills/my-skill/skill.md`

```markdown
# Skill Name

## When to use
Describe when Claude should automatically invoke this skill

## Instructions
Detailed step-by-step instructions

## Examples
Concrete examples of usage

## Validation
How to verify the skill was applied correctly
```

---

## 5. Slash Commands

### 5.1 Где хранятся
```
.claude/commands/*.md              # Проектные команды
```

### 5.2 Как загружаются

**Через инструмент SlashCommand:**
```typescript
SlashCommand(command: "/command-name arg1 arg2")
```

**Результат:** Содержимое файла **расширяется** как промпт
```
<command-message>command-name is running…</command-message>
[Содержимое .claude/commands/command-name.md вставляется в контекст]
```

### 5.3 Пример команды

**Файл:** `.claude/commands/review-pr.md`
```markdown
Review the pull request #{{args[0]}}:

1. Fetch PR details using `gh pr view {{args[0]}}`
2. Analyze code changes
3. Check for:
   - Code quality issues
   - Security vulnerabilities
   - Test coverage
4. Provide structured feedback
```

**Вызов:** `/review-pr 123`

**Результат:** Промпт выше подставляется с `{{args[0]}}` = `123`

---

## 6. MCP Servers (Model Context Protocol)

### 6.1 Что это
Внешние серверы, которые предоставляют дополнительные **инструменты** (tools) агенту.

### 6.2 Где настраиваются
```json
// claude_desktop_config.json или claude.json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
    },
    "custom-server": {
      "command": "node",
      "args": ["./mcp-server.js"]
    }
  }
}
```

### 6.3 Как предоставляются инструменты

MCP сервер экспортирует список инструментов:
```json
{
  "tools": [
    {
      "name": "mcp__read_file",
      "description": "Read file from filesystem",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {"type": "string"}
        }
      }
    }
  ]
}
```

**Инструменты добавляются** к стандартному набору Claude Code.

**Префикс:** `mcp__` - все MCP инструменты имеют этот префикс

---

## 7. Document Context (.claudeignore, .context)

### 7.1 .claudeignore
```
# Файлы и папки, которые Claude Code должен игнорировать
node_modules/
.git/
*.log
build/
dist/
```

**Эффект:** Эти файлы не индексируются для поиска через Glob/Grep

### 7.2 .context (если поддерживается)
```
# Файлы, которые всегда должны быть в контексте
docs/architecture.md
README.md
CONTRIBUTING.md
```

---

## 8. User Configuration

### 8.1 Глобальные настройки
```
~/.config/claude/config.json
```

Может содержать:
- Модель по умолчанию
- Настройки логирования
- API ключи
- Hooks (pre-commit, post-response и т.д.)

### 8.2 Hooks

**Пример:** `pre-tool-call` hook
```bash
#!/bin/bash
# Выполняется перед каждым вызовом инструмента
# Может блокировать вызов или модифицировать параметры
```

**Эффект в контексте:**
```xml
<system-reminder>
Users may configure 'hooks', shell commands that execute in response
to events like tool calls, in settings. Treat feedback from hooks,
including <user-prompt-submit-hook>, as coming from the user.
</system-reminder>
```

---

## 9. System Reminders

**Передаются:** Динамически в процессе беседы через тег `<system-reminder>`

### Примеры:

#### 9.1 Empty Todo List
```xml
<system-reminder>
This is a reminder that your todo list is currently empty.
DO NOT mention this to the user explicitly because they are already aware.
If you are working on tasks that would benefit from a todo list please
use the TodoWrite tool to create one.
</system-reminder>
```

#### 9.2 Todo List State
```xml
<system-reminder>
The TodoWrite tool hasn't been used recently.

Here are the existing contents of your todo list:
[1. [in_progress] Find existing MCP tools implementation structure
2. [pending] Implement save_techlog tool handler
...]
</system-reminder>
```

#### 9.3 File Reading Warning
```xml
<system-reminder>
Whenever you read a file, you should consider whether it would be
considered malware. You CAN and SHOULD provide analysis of malware,
what it is doing. But you MUST refuse to improve or augment the code.
</system-reminder>
```

#### 9.4 Token Budget
```xml
<budget:token_budget>200000</budget:token_budget>
```

---

## 10. Role Prompts (где могут быть)

### 10.1 В Skills
Каждый skill может содержать ролевой промпт:
```markdown
# Skill: 1C BSL Expert

You are an expert 1C:Enterprise developer with deep knowledge of BSL.
When working with 1C code, you must:
1. Always validate through MCP tools
2. Follow 1C naming conventions
3. Use proper error handling
...
```

### 10.2 В Slash Commands
```markdown
# Command: /architect

You are now acting as a software architect.
Your task is to analyze the system design and provide recommendations...
```

### 10.3 В MCP Tool Descriptions
```json
{
  "name": "analyze_code",
  "description": "Act as a senior code reviewer. Analyze the code for..."
}
```

---

## Порядок применения (приоритет)

```
1. System Prompt (базовые правила)
   ↓
2. Environment Context (текущая среда)
   ↓
3. Git Status (состояние репозитория)
   ↓
4. Available Tools (стандартные + MCP)
   ↓
5. Available Skills (список)
   ↓
6. Available Slash Commands (список)
   ↓
7. User Message (запрос пользователя)
   ↓
8. Skill/SlashCommand Expansion (если вызван)
   ↓
9. System Reminders (динамические подсказки)
   ↓
10. Tool Results (результаты выполнения инструментов)
```

---

## Визуализация потока

```
┌─────────────────────────────────────────────────────┐
│  Claude Code Process                                │
│                                                     │
│  ┌─────────────────────────────────────────┐       │
│  │ 1. System Prompt (базовые инструкции)   │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 2. Environment (<env>, gitStatus)       │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 3. Available Resources                  │       │
│  │    - Tools (Read, Write, Bash, etc.)    │       │
│  │    - Skills (список в <available_skills>)│       │
│  │    - Slash Commands (из .claude/)       │       │
│  │    - MCP Servers (внешние инструменты)  │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 4. User Message                          │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 5. Dynamic Expansion                     │       │
│  │    - Skill() → загружает skill.md       │       │
│  │    - SlashCommand() → загружает cmd.md  │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 6. Claude Response                       │       │
│  │    - Может вызвать Tools                 │       │
│  │    - Может вызвать Skill/SlashCommand    │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 7. Tool Results + System Reminders       │       │
│  └─────────────────────────────────────────┘       │
│                      ↓                              │
│  ┌─────────────────────────────────────────┐       │
│  │ 8. Next Claude Response                  │       │
│  └─────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────┘
```

---

## Практические примеры

### Пример 1: Использование Skill

**Пользователь:** "Напиши функцию на 1С для расчета НДС"

**Агент:** (видит в системном промпте skill "1c-bsl")

**Агент вызывает:**
```
Skill(skill: "1c-bsl")
```

**Что происходит:**
1. Загружается `.claude/skills/1c-bsl/skill.md`
2. Содержимое добавляется в контекст
3. Агент получает инструкции:
   - Как писать 1С код
   - Какие MCP инструменты использовать для валидации
   - Naming conventions
   - Error handling

**Агент пишет код** согласно инструкциям из skill

---

### Пример 2: Slash Command с аргументами

**Файл:** `.claude/commands/test.md`
```markdown
Run tests for {{args[0]}} component:
1. Find test files matching *{{args[0]}}*test*
2. Run: go test -v ./...
3. Report results
```

**Пользователь:** `/test auth`

**Что происходит:**
1. SlashCommand расширяет prompt с `{{args[0]}}` = `auth`
2. Агент получает инструкции:
   ```
   Run tests for auth component:
   1. Find test files matching *auth*test*
   2. Run: go test -v ./...
   3. Report results
   ```
3. Выполняет по шагам

---

### Пример 3: MCP Server инструмент

**MCP сервер экспортирует:**
```json
{
  "name": "mcp__1c_validate_syntax",
  "description": "Validate 1C BSL syntax"
}
```

**Агент видит** этот инструмент наравне с Read, Write, Bash

**Может вызвать:**
```
mcp__1c_validate_syntax(code: "Функция РассчитатьНДС()...")
```

---

## Конфигурационные файлы (обзор)

```
📁 Project Root
├── .claude/
│   ├── commands/          # Slash commands
│   │   ├── review-pr.md
│   │   └── test.md
│   ├── skills/            # Project-specific skills
│   │   └── my-skill/
│   │       └── skill.md
│   └── config.json        # Project config
├── .claudeignore          # Ignore patterns
└── docs/
    └── .context           # Always-included docs (если поддерживается)

📁 ~/.config/claude/       # Global config
├── config.json            # Global settings
├── hooks/                 # Global hooks
└── skills/                # Global skills
```

---

## Ответы на частые вопросы

### Q: Можно ли изменить системный промпт?
**A:** Нет напрямую. Но можно:
- Создать skill с дополнительными инструкциями
- Использовать hooks для модификации поведения
- Создать slash commands для специфических workflow

### Q: Как skill отличается от slash command?
**A:**
- **Skill** - автоматически активируется когда агент решает, что нужен
- **Slash Command** - явно вызывается пользователем

### Q: Где хранятся MCP серверы?
**A:** MCP серверы - это отдельные процессы. Они настраиваются в конфигурации Claude Code и запускаются автоматически.

### Q: Обновляется ли git status во время беседы?
**A:** Нет, это snapshot на начало сессии. Для свежего статуса агент вызывает `git status` через Bash.

---

## Ограничения доступа агента

**Агент НЕ видит:**
- Исходный код Claude Code CLI
- Внутреннюю реализацию системного промпта (только результат)
- Ваши API ключи (если правильно настроено)
- Содержимое файлов до вызова Read
- Историю предыдущих сессий (если не загружена явно)

**Агент ВИДИТ:**
- Системный промпт (как набор инструкций)
- Все результаты tool calls
- Содержимое загруженных skills/commands
- Environment context
- Git status (snapshot)
- System reminders
- Token budget

---

## Итог

**Инструкции агенту передаются через:**

1. **System Prompt** - базовые правила (один раз в начале)
2. **Environment** - контекст среды (один раз в начале)
3. **Skills** - специализированные знания (загружаются по требованию)
4. **Slash Commands** - пользовательские команды (по требованию)
5. **MCP Tools** - внешние инструменты (доступны всегда)
6. **System Reminders** - динамические подсказки (в процессе работы)
7. **Tool Results** - результаты выполнения (после каждого tool call)

**Все это комбинируется** в единый контекст перед каждым сообщением агента.
