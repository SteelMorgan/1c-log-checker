# Анализ полей технологического журнала

## Список всех полей из навыка

### Общие свойства (присутствуют во всех событиях)
- `name` - тип события (EXCP, DBMSSQL, TLOCK и т.д.)
- `level` - уровень (TRACE, DEBUG, INFO, WARNING, ERROR)
- `duration` - длительность в микросекундах
- `process` - имя процесса (rphost, 1cv8c и т.д.)
- `OSThread` - ID потока ОС
- `depth` - уровень вложенности в стеке вызовов

### Общие свойства (могут отсутствовать)
- `ClientID` - ID клиента
- `SessionID` - ID сеанса
- `Usr` - имя пользователя 1С
- `AppID` - ID приложения
- `ConnID` - ID соединения
- `TransactionID` - ID транзакции
- `Interface` - интерфейс
- `Method` - метод
- `CallID` - ID вызова

### Свойства SQL-событий (DBMSSQL, DBPOSTGRS, DBORACLE, DB2, SDBL)
- `sql` - текст SQL-запроса ⭐ **КРИТИЧНО ДЛЯ ЗАПРОСОВ**
- `planSQLText` - план выполнения запроса ⭐ **КРИТИЧНО ДЛЯ ЗАПРОСОВ**
- `Rows` - количество возвращенных строк ⭐ **ЧАСТО ИСПОЛЬЗУЕТСЯ**
- `RowsAffected` - количество измененных строк ⭐ **ЧАСТО ИСПОЛЬЗУЕТСЯ**
- `Database` - имя базы данных
- `Dbms` - тип СУБД (DBMSSQL, DBPOSTGRS и т.д.)

### Свойства блокировок (TLOCK, TTIMEOUT, TDEADLOCK)
- `Locks` - список управляемых блокировок
- `Regions` - имена областей блокировок
- `WaitConnections` - соединения, которые блокируются
- `lka` - поток является источником блокировки
- `lkp` - поток является жертвой блокировки
- `lkpid` - номер запроса жертвы
- `lkaid` - номера запросов источника
- `lksrc` - номер соединения источника
- `lkpto` - время ожидания жертвы
- `lkato` - время блокировки источником

### Свойства исключений (EXCP, EXCPCNTX)
- `Exception` - имя исключения ⭐ **КРИТИЧНО ДЛЯ ДИАГНОСТИКИ**
- `Descr` - описание исключения ⭐ **КРИТИЧНО ДЛЯ ДИАГНОСТИКИ**
- `Context` - контекст выполнения (стек вызовов) ⭐ **КРИТИЧНО ДЛЯ ДИАГНОСТИКИ**

### Другие возможные свойства
- `Computer` - имя компьютера
- `Server` - имя сервера
- `Port` - порт
- `File` - имя файла
- `Line` - номер строки
- `Module` - имя модуля
- `Function` - имя функции
- И другие динамические свойства...

## Текущая схема tech_log в ClickHouse

### Основные колонки (фиксированные)
- `ts` - DateTime64(6) ✅
- `duration` - UInt64 ✅
- `name` - LowCardinality(String) ✅
- `level` - LowCardinality(String) ✅
- `depth` - UInt8 ✅
- `process` - LowCardinality(String) ✅
- `os_thread` - UInt32 ✅
- `client_id` - UInt64 ✅
- `session_id` - String ✅
- `transaction_id` - String ✅
- `usr` - String ✅
- `app_id` - String ✅
- `connection_id` - UInt64 ✅
- `interface` - String ✅
- `method` - String ✅
- `call_id` - UInt64 ✅
- `cluster_guid` - String ✅
- `infobase_guid` - String ✅
- `raw_line` - String ✅
- `record_hash` - String ✅

### Динамические свойства
- `property_key` - Array(String) ✅
- `property_value` - Array(String) ✅

## Анализ: недостающие колонки

### ⚠️ КРИТИЧНО: Часто используемые поля хранятся только в property_key/property_value

Для эффективных запросов нужно добавить отдельные колонки для:

1. **SQL-запросы** (очень часто используются в анализе производительности):
   - `sql` - String (может быть очень длинным)
   - `plan_sql_text` - String (может быть очень длинным)
   - `rows` - UInt64
   - `rows_affected` - UInt64
   - `dbms` - LowCardinality(String)
   - `database` - String

2. **Исключения** (критично для диагностики ошибок):
   - `exception` - String
   - `exception_descr` - String
   - `exception_context` - String (может быть очень длинным)

3. **Блокировки** (важно для анализа блокировок):
   - `locks` - String (JSON или список)
   - `regions` - String (JSON или список)
   - `wait_connections` - String (JSON или список)

## Рекомендация

Добавить отдельные колонки для часто используемых полей, чтобы:
- Ускорить запросы (не нужно парсить property_key/property_value)
- Упростить фильтрацию (WHERE sql LIKE '%...%')
- Улучшить индексацию (можно добавить индексы на sql, exception и т.д.)

Остальные свойства останутся в property_key/property_value для гибкости.

