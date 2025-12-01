# Инструкции по откату изменений

**Дата создания:** 2025-01-XX  
**Тег для отката:** `before-1clogs-update`  
**Commit:** `637f6ea` - "Snapshot before applying 1cLogs improvements - checkpoint for rollback"

---

## 📌 Текущее состояние зафиксировано

Перед применением доработок из проекта `1cLogs` было создано:
- ✅ **Git commit** с текущим состоянием
- ✅ **Git tag** `before-1clogs-update` для удобного отката

---

## 🔄 Способы отката

### Способ 1: Откат к тегу (рекомендуется)

```powershell
# Перейти в директорию проекта
cd "D:\My Projects\FrameWork 1C\1c-log-checker"

# Откатиться к состоянию до обновления
git checkout before-1clogs-update

# Если нужно создать новую ветку из этого состояния
git checkout -b rollback-branch before-1clogs-update
```

### Способ 2: Откат к конкретному commit

```powershell
# Откатиться к commit 637f6ea
git checkout 637f6ea

# Или создать новую ветку
git checkout -b rollback-branch 637f6ea
```

### Способ 3: Hard reset (⚠️ ОСТОРОЖНО - удалит все незакоммиченные изменения)

```powershell
# Откатиться и удалить все изменения после commit
git reset --hard 637f6ea

# Или к тегу
git reset --hard before-1clogs-update
```

### Способ 4: Откат через revert (создаст новый commit)

```powershell
# Если изменения уже закоммичены, можно откатить через revert
git revert HEAD

# Или откатить несколько последних коммитов
git revert HEAD~3..HEAD
```

---

## 📋 Проверка текущего состояния

### Посмотреть информацию о теге:
```powershell
git show before-1clogs-update
```

### Посмотреть список коммитов:
```powershell
git log --oneline -10
```

### Посмотреть текущую ветку и статус:
```powershell
git status
git branch
```

---

## ⚠️ Важные замечания

1. **Перед откатом сохраните важные изменения:**
   - Если есть незакоммиченные изменения, которые нужно сохранить:
     ```powershell
     git stash save "Важные изменения перед откатом"
     ```

2. **После отката восстановить изменения:**
   ```powershell
   git stash list
   git stash pop
   ```

3. **Если изменения уже запушены в remote:**
   - Откат локально не затронет remote
   - Для отката remote нужно использовать `git push --force` (⚠️ ОСТОРОЖНО!)

4. **Создание бэкапа перед откатом:**
   ```powershell
   # Скопировать всю директорию проекта
   Copy-Item "D:\My Projects\FrameWork 1C\1c-log-checker" -Destination "D:\My Projects\FrameWork 1C\1c-log-checker-backup" -Recurse
   ```

---

## 📝 Что было зафиксировано

### Commit: `637f6ea`
**Сообщение:** "Snapshot before applying 1cLogs improvements - checkpoint for rollback"

**Измененные файлы:**
- README_AI.md
- deploy/clickhouse/init/*.sql (5 файлов)
- deploy/grafana/provisioning/dashboards/event-log-temporal.json
- docs/*.md (5 файлов)
- internal/handlers/tracing.go
- scripts/test_event_log_verification.ps1
- COMPARISON_REPORT.md (новый файл)

---

## 🎯 Быстрый откат (одна команда)

```powershell
cd "D:\My Projects\FrameWork 1C\1c-log-checker"; git checkout before-1clogs-update
```

---

## 📞 Если что-то пошло не так

1. **Проверить текущее состояние:**
   ```powershell
   git status
   git log --oneline -5
   ```

2. **Посмотреть все теги:**
   ```powershell
   git tag -l
   ```

3. **Посмотреть информацию о commit:**
   ```powershell
   git show 637f6ea
   ```

4. **Восстановить из stash (если использовали):**
   ```powershell
   git stash list
   git stash show -p stash@{0}  # Посмотреть изменения
   git stash pop  # Применить изменения
   ```

---

**Дата создания:** 2025-01-XX  
**Статус:** ✅ Готово к применению доработок








