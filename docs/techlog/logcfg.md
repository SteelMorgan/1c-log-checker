# Настройка технологического журнала (logcfg.xml)

Руководство по конфигурации технологического журнала 1С:Предприятие.

---

## Расположение файла

**Windows:**
```
C:\Program Files\1cv8\conf\logcfg.xml
```

Для 32-битной платформы:
```
C:\Program Files (x86)\1cv8\conf\logcfg.xml
```

**Linux:**
```
~/.1cv8/conf/logcfg.xml
```

---

## Базовая структура

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="D:\1CLogs\" history="24">
        <event>
            <eq property="name" value="excp"/>
        </event>
        <property name="all"/>
    </log>
</config>
```

---

## Атрибуты элемента `<log>`

| Атрибут | Описание | Значения | По умолчанию |
|---------|----------|----------|--------------|
| `location` | Каталог для файлов логов | Путь к папке | - |
| `history` | Время хранения (часы) | 1-... | 24 |
| `format` | Формат файлов | `text`, `json` | `text` |
| `placement` | Структура хранения | `folders`, `plain` | `folders` |
| `rotation` | Схема ротации | `period`, `size` | `period` |
| `rotationperiod` | Период ротации (часы) | 1-... | 1 |
| `rotationsize` | Размер для ротации (МБ) | 1-... | 100 |
| `compress` | Сжатие файлов | `none`, `zip` | `none` |

---

## События (40+ типов)

### Критичные для диагностики ошибок:

| Событие | Описание | Когда использовать |
|---------|----------|-------------------|
| `EXCP` | Исключительные ситуации | Всегда при отладке |
| `EXCPCNTX` | Контекст исключения | Вместе с EXCP |
| `QERR` | Ошибки компиляции запросов | Проблемы с запросами |

### События производительности:

| Событие | Описание | Объём логов |
|---------|----------|-------------|
| `DBMSSQL` | SQL к MS SQL Server | Большой ⚠️ |
| `DBPOSTGRS` | SQL к PostgreSQL | Большой ⚠️ |
| `DBORACLE` | SQL к Oracle | Большой ⚠️ |
| `SDBL` | Запросы к модели 1С | Большой ⚠️ |

**⚠️ Важно:** Используйте фильтры по `duration` для этих событий!

### События блокировок:

| Событие | Описание |
|---------|----------|
| `TLOCK` | Управление блокировками |
| `TTIMEOUT` | Превышение времени ожидания |
| `TDEADLOCK` | Взаимоблокировка |

### События соединений:

| Событие | Описание |
|---------|----------|
| `CONN` | Соединения клиентов |
| `SESN` | Сеансы пользователей |
| `PROC` | События процесса |
| `SCOM` | Серверный контекст |

Полный список: см. документацию платформы (40+ событий).

---

## Фильтры событий

### Операторы сравнения:

```xml
<!-- Равно -->
<eq property="name" value="excp"/>

<!-- Не равно -->
<ne property="level" value="INFO"/>

<!-- Больше (для duration в микросекундах) -->
<gt property="duration" value="1000000"/>  <!-- >1 сек -->

<!-- Больше или равно -->
<ge property="duration" value="500000"/>

<!-- Меньше -->
<lt property="duration" value="10000000"/>

<!-- Меньше или равно -->
<le property="duration" value="5000000"/>

<!-- По маске -->
<like property="Txt" value="%ошибка%"/>
```

### Примеры фильтров:

**Только медленные SQL-запросы:**
```xml
<event>
    <eq property="name" value="dbmssql"/>
    <gt property="duration" value="3000000"/>  <!-- >3 сек -->
</event>
```

**Ошибки конкретного пользователя:**
```xml
<event>
    <eq property="name" value="excp"/>
    <eq property="Usr" value="Иванов"/>
</event>
```

---

## Свойства (properties)

### Все свойства:
```xml
<property name="all"/>
```

### Конкретные свойства:
```xml
<property name="sql"/>           <!-- Текст SQL-запроса -->
<property name="planSQLText"/>   <!-- План выполнения -->
<property name="duration"/>      <!-- Длительность -->
<property name="Rows"/>          <!-- Количество строк -->
```

### Условные свойства:
```xml
<!-- SQL только для медленных запросов -->
<property name="sql">
    <event>
        <eq property="name" value="dbmssql"/>
        <gt property="duration" value="1000000"/>
    </event>
</property>
```

---

## Примеры конфигураций

### 1. Минимальная (только ошибки)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="D:\1CLogs\Errors\" history="120">
        <event>
            <eq property="name" value="excp"/>
        </event>
        <event>
            <eq property="name" value="qerr"/>
        </event>
        <property name="all"/>
    </log>
</config>
```

### 2. Диагностика блокировок

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="D:\1CLogs\Locks\" history="48">
        <event>
            <eq property="name" value="tlock"/>
        </event>
        <event>
            <eq property="name" value="ttimeout"/>
        </event>
        <event>
            <eq property="name" value="tdeadlock"/>
        </event>
        <event>
            <eq property="name" value="conn"/>
        </event>
        <event>
            <eq property="name" value="sesn"/>
        </event>
        <property name="all"/>
    </log>
    <dbmslocks/>  <!-- Включить сбор инфо о блокировках СУБД -->
</config>
```

### 3. Анализ производительности

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="D:\1CLogs\Performance\" history="12" format="json">
        <event>
            <eq property="name" value="dbmssql"/>
            <gt property="duration" value="1000000"/>  <!-- >1 сек -->
        </event>
        <event>
            <eq property="name" value="sdbl"/>
            <gt property="duration" value="500000"/>  <!-- >0.5 сек -->
        </event>
        <property name="sql"/>
        <property name="planSQLText"/>
        <property name="Context"/>
        <property name="Rows"/>
    </log>
    <plansql/>  <!-- Включить сбор планов запросов -->
</config>
```

### 4. Полная конфигурация (для критичных проблем)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="true" type="2" location="D:\1CLogs\Dumps\"/>
    <log location="D:\1CLogs\Full\" history="6" format="json" compress="zip">
        <event>
            <eq property="name" value="excp"/>
        </event>
        <event>
            <eq property="name" value="conn"/>
        </event>
        <event>
            <eq property="name" value="proc"/>
        </event>
        <event>
            <eq property="name" value="scom"/>
        </event>
        <event>
            <eq property="name" value="dbmssql"/>
        </event>
        <event>
            <eq property="name" value="sdbl"/>
        </event>
        <event>
            <eq property="name" value="tlock"/>
        </event>
        <property name="all"/>
    </log>
    <dbmslocks/>
    <plansql/>
</config>
```

---

## Best Practices

### ✅ Рекомендуется:

1. **Ограничивать history** — 12-48 часов для production
2. **Использовать фильтры duration** для SQL-событий
3. **Включать compress="zip"** для экономии места
4. **Использовать SSD** для каталога логов
5. **Мониторить размер логов** — может быстро расти
6. **Отключать после решения проблемы** — не оставлять включённым постоянно

### ❌ Не рекомендуется:

1. Более 20 элементов `<log>` в одном файле
2. Логировать все DBMSSQL без фильтра duration
3. history > 120 часов в production
4. Включать SYSTEM events без указания техподдержки
5. Размещать логи на системном диске

---

## Применение через MCP

После генерации конфигурации через `configure_techlog`:

1. Сохранить результат в `C:\Program Files\1cv8\conf\logcfg.xml`
2. Перезапустить кластер 1С или дождаться автоприменения (проверяется раз в минуту)
3. Проверить появление файлов в указанном `location`
4. После решения проблемы — вызвать `disable_techlog`

---

## Источники

- [Детальная статья про техжурнал (Infostart)](https://infostart.ru/1c/articles/1195695/)
- Документация платформы 1С (Приложение 3, раздел 3.24)
- [TODO: Навык "Технологический журнал"](../guides/TODO_techlog_skill.md)

---

**Следующие шаги:** После настройки logcfg.xml используйте `get_tech_log` для просмотра результатов.

