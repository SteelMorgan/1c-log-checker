# Contributing to 1C Log Parser Service

Добро пожаловать! Этот проект следует методологии Kiro с формальной спецификацией и процессом разработки.

---

## Процесс работы

### Обязательные шаги

1. **Прочитайте спецификацию**: [docs/specs/log-service.spec.md](docs/specs/log-service.spec.md)
2. **Изучите процесс Киры**: [docs/specs/workflow-process.md](docs/specs/workflow-process.md)
3. **Следуйте чек-листу**: [docs/specs/kiro-checklist.md](docs/specs/kiro-checklist.md)

### Добавление нового функционала

1. **Создайте требование** в разделе Requirements спеки
2. **Дождитесь утверждения** от владельца проекта
3. **Разработайте дизайн** и добавьте в раздел Design
4. **Получите утверждение дизайна**
5. **Создайте задачи** в Implementation Tasks
6. **Реализуйте** по утверждённым задачам
7. **Проверьте чек-лист** перед созданием PR

---

## Стандарты кода

### Go

Проект следует правилам из [.cursor/rules/GO.MDC](.cursor/rules/GO.MDC):

- **Clean Architecture** (handlers → services → repositories → domain)
- **Интерфейсы** для всех зависимостей
- **Dependency Injection** через конструкторы
- **Table-driven tests** для unit-тестов
- **GoDoc комментарии** для экспортируемых функций
- **Обработка ошибок** с wrap (`fmt.Errorf("context: %w", err)`)
- **Context propagation** для cancellation

### Линтинг

Перед коммитом запустите:

```powershell
golangci-lint run
goimports -w .
go fmt ./...
```

### Тестирование

Обязательные тесты:

- **Unit tests**: покрытие >80% для exported functions
- **Integration tests**: для ClickHouse, BoltDB
- **E2E tests**: для критических путей

Запуск:

```powershell
go test ./... -v
go test ./... -cover
```

---

## Структура коммитов

Формат:

```
<type>: <summary>

<body>

<footer>
```

**Types:**
- `feat`: новый функционал
- `fix`: исправление ошибки
- `docs`: изменения документации
- `refactor`: рефакторинг без изменения поведения
- `test`: добавление/изменение тестов
- `chore`: обновление зависимостей, конфигов

**Пример:**

```
feat: implement event log reader for .lgf/.lgp formats

- Added EventLogReader interface
- Implemented .lgf header parser
- Implemented .lgp fragment reader
- Added deduplication logic
- Unit tests with table-driven approach

Closes #42
```

---

## Pull Request процесс

1. **Создайте ветку** от `main`:
   ```powershell
   git checkout -b feature/your-feature-name
   ```

2. **Реализуйте** изменения согласно спеке

3. **Протестируйте**:
   ```powershell
   go test ./...
   golangci-lint run
   ```

4. **Обновите документацию** (если применимо)

5. **Создайте PR** с описанием:
   - Ссылка на требование в спеке
   - Краткое описание изменений
   - Скриншоты (для UI изменений)
   - Результаты тестов

6. **Пройдите чек-лист Киры**

---

## Checklist перед PR

- [ ] Код следует правилам GO.MDC
- [ ] Все тесты проходят (`go test ./...`)
- [ ] Линтер не выдаёт ошибок (`golangci-lint run`)
- [ ] Добавлены GoDoc комментарии
- [ ] Обновлена документация
- [ ] Changelog обновлён в спеке
- [ ] Чек-лист Киры пройден

---

## Вопросы и поддержка

Если у вас вопросы:
1. Проверьте [docs/specs/log-service.spec.md](docs/specs/log-service.spec.md)
2. Изучите [README.md](README.md)
3. Создайте issue с меткой `question`

---

Спасибо за вклад в проект!

