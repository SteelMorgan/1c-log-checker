# Руководство по сравнению результатов парсинга

При тестировании парсера журнала регистрации важно сравнить результаты с данными, видимыми в конфигураторе 1С.

## Поля для сравнения

### Основные поля (Primary View в конфигураторе)

| Поле в конфигураторе | Поле в парсере | Примечание |
|---------------------|----------------|------------|
| **Дата, время** | `event_time` | Формат: `02.01.2006 15:04:05` |
| **Уровень** | `level` | Информация, Предупреждение, Ошибка, Примечание |
| **Событие** | `event_presentation` | Полное представление (например, "Сеанс.Начало") |
| **Пользователь** | `user_name` | Имя пользователя |
| **Компьютер** | `computer` | Имя компьютера |
| **Приложение** | `application_presentation` | Толстый клиент, Тонкий клиент, Конфигуратор и т.д. |
| **Сеанс** | `session_id` | Номер сеанса |
| **Статус транзакции** | `transaction_status` | Зафиксирована, Отменена, Не завершена, Нет транзакции |
| **Комментарий** | `comment` | Описание события |
| **Метаданные** | `metadata_presentation` | Представление метаданных |
| **Представление данных** | `data_presentation` | Представление данных (ссылка на объект) |

### Дополнительные поля

- **Транзакция** → `transaction_id`
- **Разделение данных сеанса** → `data_separation`
- **Соединение** → `connection_id`
- **Сервер** → `server`
- **Основной порт** → `primary_port`
- **Вспомогательный порт** → `secondary_port`

## Запуск тестирования

### 1. Запуск в READ_ONLY режиме

```powershell
$env:EVENT_LOG_METHOD="direct"
$env:READ_ONLY="true"
$env:LOG_DIRS="C:\Program Files\1cv8\srvinfo"
$env:LOG_LEVEL="info"
go run ./cmd/parser
```

### 2. Поиск записей по базе

В логах ищите записи по `infobase_guid`:

```powershell
# Для базы ai_dssl_ut нужно найти её GUID
# Обычно он находится в пути: C:\Program Files\1cv8\srvinfo\reg_<port>\<infobase_guid>\1Cv8Log\
```

### 3. Сравнение последних записей

Парсер выводит каждую запись в формате:
```
event_time=13.11.2025 13:44:00
level=Информация
event=_$Session$_.Start
event_presentation=Сеанс.Начало
user_name=...
computer=STEEL-PC
application_presentation=Конфигуратор
session_id=1
transaction_status=Нет транзакции
comment=...
```

Сравните эти значения с тем, что видно в конфигураторе 1С.

## Типичные проблемы

### 1. Несоответствие времени

- Проверьте часовой пояс
- Убедитесь, что `event_time` соответствует времени в конфигураторе

### 2. Пустые поля

- `user_name` может быть пустым, если пользователь не определен
- `metadata_presentation` может быть пустым для системных событий
- `data_presentation` может быть пустым для событий без данных

### 3. Несоответствие представлений

- Проверьте маппинг в `getEventPresentation()`, `getApplicationPresentation()`
- Убедитесь, что коды событий правильно распознаются

## Формат вывода для сравнения

Для удобного сравнения можно использовать SQL запрос в ClickHouse (после записи):

```sql
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    transaction_status as "Статус транзакции",
    comment as "Комментарий",
    metadata_presentation as "Метаданные",
    data_presentation as "Представление данных"
FROM logs.event_log
WHERE infobase_guid = '<your-infobase-guid>'
ORDER BY event_time DESC
LIMIT 50;
```









