# Docker Compose Configuration

## Настройка часового пояса

**Важно:** Переменная `TZ` используется для двух целей:
1. **Часовой пояс контейнеров** — для системных функций (date, now() в ClickHouse)
2. **Часовой пояс логов 1С** — для правильного парсинга временных меток из журналов регистрации и технологических журналов

Парсер интерпретирует временные метки из логов 1С как локальное время сервера в указанном часовом поясе и конвертирует их в UTC для хранения в ClickHouse.

### Автоматическое определение (рекомендуется)

```powershell
# Запуск с автоматическим определением часового пояса
cd deploy/docker
. .\detect-timezone.ps1
docker-compose up -d
```

### Ручная настройка

Создайте файл `.env` в директории `deploy/docker/`:

```env
TZ=Europe/Moscow
```

Или установите переменную окружения перед запуском:

```powershell
$env:TZ = "Europe/Moscow"
docker-compose up -d
```

### Поддерживаемые часовые пояса

Используйте формат IANA (например: `Europe/Moscow`, `UTC`, `America/New_York`, `Asia/Tokyo`).

Если переменная `TZ` не установлена:
- Контейнеры будут использовать `UTC` по умолчанию
- Парсер будет интерпретировать временные метки из логов как UTC (может привести к неправильному времени, если сервер 1С работает в другом часовом поясе)

### Примеры

**Московское время:**
```env
TZ=Europe/Moscow
```

**UTC:**
```env
TZ=UTC
```

**Другой часовой пояс:**
```env
TZ=Asia/Yekaterinburg
```

## Запуск контейнеров

```powershell
cd deploy/docker
docker-compose up -d
```

## Остановка контейнеров

```powershell
cd deploy/docker
docker-compose down
```

## Пересборка сервисов

```powershell
cd deploy/docker
docker-compose build [service-name]
docker-compose up -d [service-name]
```

