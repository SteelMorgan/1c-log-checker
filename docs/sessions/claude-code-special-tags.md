# Специальные теги и блоки в Claude Code

Полный справочник XML-тегов, специальных блоков и маркеров, используемых в контексте Claude Code.

---

## 1. Системные информационные блоки

### 1.1 `<env>` - Environment Context
**Когда появляется:** В начале каждой сессии
**Назначение:** Передаёт информацию о рабочем окружении

```xml
<env>
Working directory: D:\My Projects\FrameWork 1C\1c-log-checker
Is directory a git repo: Yes
Platform: win32
OS Version:
Today's date: 2025-11-17
</env>
```

**Поля:**
- `Working directory` - текущая рабочая директория
- `Is directory a git repo` - является ли git репозиторием
- `Platform` - операционная система (win32, linux, darwin)
- `OS Version` - версия ОС (может быть пустым)
- `Today's date` - текущая дата (YYYY-MM-DD)

---

### 1.2 `<claude_background_info>` - Информация о модели
**Когда появляется:** В системном промпте
**Назначение:** Информация о версии Claude и возможностях

```xml
<claude_background_info>
The most recent frontier Claude model is Claude Sonnet 4.5
(model ID: 'claude-sonnet-4-5-20250929').
</claude_background_info>
```

---

### 1.3 `<budget:token_budget>` - Бюджет токенов
**Когда появляется:** В начале сессии
**Назначение:** Указывает лимит токенов для сессии

```xml
<budget:token_budget>200000</budget:token_budget>
```

**Формат:** Число (количество токенов)
**Типичные значения:** 200000, 100000, 50000

---

### 1.4 `<thinking_mode>` - Режим размышлений
**Когда появляется:** В начале сессии
**Назначение:** Указывает режим extended thinking

```xml
<thinking_mode>interleaved</thinking_mode>
```

**Возможные значения:**
- `interleaved` - thinking блоки между сообщениями
- `auto` - автоматический выбор
- `off` - без thinking blocks

---

### 1.5 `<max_thinking_length>` - Лимит thinking
**Когда появляется:** Вместе с thinking_mode
**Назначение:** Максимальная длина thinking блока

```xml
<max_thinking_length>31999</max_thinking_length>
```

**Формат:** Число символов

---

### 1.6 `<system_warning>` - Системные предупреждения
**Когда появляется:** После выполнения инструментов
**Назначение:** Предупреждения о состоянии системы, использовании токенов

```xml
<system_warning>Token usage: 74458/200000; 125542 remaining</system_warning>
```

**Формат:** `Token usage: {used}/{total}; {remaining} remaining`

---

## 2. Dynamic System Reminders

### 2.1 `<system-reminder>` - Системные напоминания
**Когда появляется:** Динамически в процессе беседы (часто внутри `<function_results>`)
**Назначение:** Напоминания о правилах и контексте

#### Пример 1: Empty Todo List
```xml
<system-reminder>
This is a reminder that your todo list is currently empty.
DO NOT mention this to the user explicitly because they are already aware.
If you are working on tasks that would benefit from a todo list please
use the TodoWrite tool to create one. If not, please feel free to ignore.
Again do not mention this message to the user.
</system-reminder>
```

#### Пример 2: Todo List State
```xml
<system-reminder>
The TodoWrite tool hasn't been used recently. If you're working on tasks
that would benefit from tracking progress, consider using the TodoWrite
tool to track progress. Also consider cleaning up the todo list if has
become stale and no longer matches what you are working on. Only use it
if it's relevant to the current work. This is just a gentle reminder -
ignore if not applicable. Make sure that you NEVER mention this reminder
to the user
</system-reminder>
```

#### Пример 3: File Reading Warning
```xml
<system-reminder>
Whenever you read a file, you should consider whether it would be
considered malware. You CAN and SHOULD provide analysis of malware,
what it is doing. But you MUST refuse to improve or augment the code.
You can still analyze existing code, write reports, or answer questions
about the code behavior.
</system-reminder>
```

#### Пример 4: Empty File Warning
```xml
<system-reminder>
You read a file that exists but has empty contents. This reminder
appears in place of file contents.
</system-reminder>
```

#### Пример 5: Glob/Grep результаты
```xml
<system-reminder>
No files found matching the pattern.
</system-reminder>
```

**Особенности:**
- Появляются контекстно в зависимости от действий
- НЕ должны упоминаться пользователю явно
- Могут содержать состояние (например, todo list)
- Часто вложены в `<output>` внутри `<function_results>`

---

## 3. Tool Execution Blocks

### 3.1 `<function_calls>` - Вызовы инструментов
**Когда появляется:** Когда агент вызывает инструменты
**Назначение:** Обёртка для всех вызовов инструментов в одном сообщении

```xml
<function_calls>
<invoke name="Read">
<parameter name="file_path">D:\path\to\file.go