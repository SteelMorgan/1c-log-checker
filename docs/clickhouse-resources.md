# Анализ ресурсов ClickHouse

## Текущее состояние

### Использование ресурсов

**CPU:**
- **Текущее использование**: ~528% (это означает использование ~5.28 ядер из 8 физических)
- **Доступно в контейнере**: 16 виртуальных ядер (видит хост)
- **Ограничения**: **НЕТ** (в docker-compose.yml не указаны)

**Память:**
- **Текущее использование**: ~1.5 GiB из 30.15 GiB (около 5%)
- **Ограничения**: **НЕТ** (max_memory_usage = 0 означает неограниченно)

### Конфигурация ClickHouse

**Потоки:**
- `max_threads`: `'auto(16)'` - автоматически использует до 16 потоков
- `background_pool_size`: 16 - пул фоновых потоков для операций слияния
- `max_concurrent_queries_for_user`: 0 (неограниченно)

**Память:**
- `max_memory_usage`: 0 (неограниченно)
- `max_server_memory_usage`: не установлено (неограниченно)

## Проблема

**ClickHouse не имеет ограничений ресурсов**, что может привести к:
1. **Перегрузке CPU** при интенсивной обработке данных
2. **Исчерпанию памяти** при больших запросах
3. **Конкуренции с другими процессами** на хосте

## Решения

### 1. Ограничение CPU в docker-compose.yml

Добавить секцию `deploy.resources.limits` для ClickHouse:

```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    container_name: 1c-log-clickhouse
    # ... существующие настройки ...
    deploy:
      resources:
        limits:
          cpus: '4'  # Ограничить до 4 ядер
        reservations:
          cpus: '2'  # Гарантировать минимум 2 ядра
```

**Рекомендации:**
- Для легкой нагрузки: `cpus: '2'`
- Для средней нагрузки: `cpus: '4'`
- Для тяжелой нагрузки: `cpus: '6'` или больше

### 2. Ограничение памяти в docker-compose.yml

```yaml
services:
  clickhouse:
    # ... существующие настройки ...
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 4G  # Ограничить память до 4 GB
        reservations:
          cpus: '2'
          memory: 2G  # Гарантировать минимум 2 GB
```

**Рекомендации:**
- Минимум: 2 GB
- Для средней нагрузки: 4-8 GB
- Для тяжелой нагрузки: 8-16 GB

### 3. Ограничение потоков в ClickHouse

Создать конфигурационный файл `deploy/clickhouse/config/config.xml`:

```xml
<?xml version="1.0"?>
<clickhouse>
    <max_threads>8</max_threads>
    <background_pool_size>8</background_pool_size>
    <max_concurrent_queries_for_user>10</max_concurrent_queries_for_user>
    <max_memory_usage>4000000000</max_memory_usage>  <!-- 4 GB в байтах -->
</clickhouse>
```

И добавить volume в docker-compose.yml:

```yaml
services:
  clickhouse:
    # ... существующие настройки ...
    volumes:
      - clickhouse_data:/var/lib/clickhouse
      - ../clickhouse/init:/docker-entrypoint-initdb.d:ro
      - ../clickhouse/config/users.xml:/etc/clickhouse-server/users.d/users.xml:ro
      - ../clickhouse/config/config.xml:/etc/clickhouse-server/config.d/custom-config.xml:ro
```

### 4. Комбинированный подход (рекомендуется)

**docker-compose.yml:**
```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    container_name: 1c-log-clickhouse
    ports:
      - "8123:8123"
      - "9000:9000"
    volumes:
      - clickhouse_data:/var/lib/clickhouse
      - ../clickhouse/init:/docker-entrypoint-initdb.d:ro
      - ../clickhouse/config/users.xml:/etc/clickhouse-server/users.d/users.xml:ro
      - ../clickhouse/config/config.xml:/etc/clickhouse-server/config.d/custom-config.xml:ro
    environment:
      CLICKHOUSE_DB: ${CLICKHOUSE_DB:-logs}
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 0
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 4G
        reservations:
          cpus: '2'
          memory: 2G
    networks:
      - internal
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "localhost:8123/ping"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
```

**deploy/clickhouse/config/config.xml:**
```xml
<?xml version="1.0"?>
<clickhouse>
    <!-- Ограничение потоков -->
    <max_threads>8</max_threads>
    <background_pool_size>8</background_pool_size>
    
    <!-- Ограничение одновременных запросов -->
    <max_concurrent_queries_for_user>10</max_concurrent_queries_for_user>
    
    <!-- Ограничение памяти (4 GB) -->
    <max_memory_usage>4000000000</max_memory_usage>
</clickhouse>
```

## Мониторинг

### Проверка текущего использования:

```bash
# CPU и память
docker stats 1c-log-clickhouse --no-stream

# Настройки ClickHouse
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT name, value FROM system.settings WHERE name LIKE '%max_thread%' OR name LIKE '%max_memory%' OR name LIKE '%background_pool%' FORMAT Pretty"
```

### Проверка ограничений Docker:

```bash
# Проверить ограничения контейнера
docker inspect 1c-log-clickhouse --format='{{json .HostConfig}}' | ConvertFrom-Json | Select-Object -Property CpuShares, Memory, MemoryReservation, CpuQuota, CpuPeriod, NanoCpus
```

## Рекомендации

1. **Начать с ограничения CPU до 4 ядер** - это снизит нагрузку, но сохранит производительность
2. **Установить лимит памяти 4-8 GB** - предотвратит исчерпание памяти
3. **Мониторить метрики** после применения ограничений
4. **Настроить ClickHouse параметры** для оптимизации под ограниченные ресурсы

## Применение изменений

После изменения `docker-compose.yml`:

```bash
# Пересоздать контейнер с новыми ограничениями
docker-compose -f deploy/docker/docker-compose.yml up -d clickhouse

# Проверить ограничения
docker inspect 1c-log-clickhouse --format='{{json .HostConfig}}' | ConvertFrom-Json | Select-Object -Property CpuShares, Memory, MemoryReservation, CpuQuota, CpuPeriod, NanoCpus
```

## Выводы

- **Текущее состояние**: ClickHouse не имеет ограничений ресурсов и может использовать все доступные CPU и память
- **Проблема**: Это может привести к перегрузке системы
- **Решение**: Установить разумные ограничения через docker-compose.yml и конфигурацию ClickHouse
- **Рекомендация**: Начать с 4 CPU и 4 GB памяти, затем корректировать по результатам мониторинга

