# Отладка парсера журнала регистрации

## Обзор

Для отладки парсера используются два файла с фактическими данными окончания журнала регистрации:
- `Тек конец ЖР_DSSL.mxl` - данные из базы DSSL
- `Тек конец ЖР_GBIG.mxl` - данные из базы GBIG

Эти файлы содержат данные в формате MOXCEL (XML), экспортированные из конфигуратора 1С. Длина полей в ячейках может быть короче, чем в оригинальном журнале (совпадение начала строки).

## Утилиты

### 1. extract_mxl - Извлечение данных из .mxl файлов

**Назначение:** Извлекает записи из .mxl файлов и сохраняет их в JSON формате для дальнейшего сравнения.

**Использование:**
```powershell
.\bin\extract_mxl.exe "Тек конец ЖР_DSSL.mxl"
.\bin\extract_mxl.exe "Тек конец ЖР_GBIG.mxl"
```

**Результат:**
- Создает JSON файлы: `Тек конец ЖР_DSSL.mxl.json` и `Тек конец ЖР_GBIG.mxl.json`
- Каждый JSON содержит массив записей с полями:
  - `DateTime` - дата и время события
  - `Fields` - массив значений полей (пользователь, компьютер, приложение, событие, комментарий, метаданные и т.д.)

### 2. compare - Сравнение данных MXL с результатами парсера

**Назначение:** Сравнивает данные из .mxl файлов (ожидаемые значения) с результатами парсинга .lgp файлов.

**Использование:**
```powershell
# Базовое сравнение (без разрешения через LGF)
.\bin\compare.exe "Тек конец ЖР_DSSL.mxl" "path/to/file.lgp"

# Сравнение с разрешением значений через LGF файл
.\bin\compare.exe "Тек конец ЖР_DSSL.mxl" "path/to/file.lgp" "path/to/1Cv8.lgf"
```

**Параметры:**
1. `mxl_file` - путь к .mxl файлу с ожидаемыми данными
2. `lgp_file` - путь к .lgp файлу для парсинга
3. `lgf_file` (опционально) - путь к .lgf файлу для разрешения значений (пользователи, компьютеры, метаданные)

**Результат:**
- Выводит сравнение каждой записи:
  - ✅ Match - записи совпадают
  - ❌ Mismatch - записи не совпадают (с указанием различий)
- Создает файл `comparison_result.json` с детальным сравнением всех записей

**Пример вывода:**
```
Comparing:
  MXL (expected): Тек конец ЖР_DSSL.mxl
  LGP (parsed):  C:\Program Files\1cv8\srvinfo\reg_1541\...\20251113144228.lgp
  LGF (resolver): C:\Program Files\1cv8\srvinfo\reg_1541\...\1Cv8.lgf

Extracted 15 records from MXL file
Parsed 15 records from LGP file

=== COMPARISON RESULTS ===

Record 1: ✅ Match
  MXL: 13.11.2025 14:42:28 | Fields: 12
  LGP: 13.11.2025 14:42:28 | User: Иванов | Event: Данные.Изменение

Record 2: ❌ Mismatch
  - user: MXL=Петров, LGP=Иванов
  - comment: MXL=Ошибка..., LGP=Успешно...

=== SUMMARY ===
Total records: 15
✅ Matches: 13
❌ Mismatches: 2
```

## Процесс отладки

### Шаг 1: Извлечение данных из MXL

```powershell
# Извлечь данные из обоих файлов
.\bin\extract_mxl.exe "Тек конец ЖР_DSSL.mxl"
.\bin\extract_mxl.exe "Тек конец ЖР_GBIG.mxl"
```

### Шаг 2: Найти соответствующие .lgp файлы

Нужно найти .lgp файлы, которые соответствуют данным из MXL файлов. Обычно это последние файлы в каталоге журнала регистрации.

```powershell
# Пример: найти последние .lgp файлы
Get-ChildItem "C:\Program Files\1cv8\srvinfo\reg_*\*\1Cv8Log\*.lgp" | 
    Sort-Object LastWriteTime -Descending | 
    Select-Object -First 2
```

### Шаг 3: Сравнение данных

```powershell
# Сравнить данные DSSL
.\bin\compare.exe `
    "Тек конец ЖР_DSSL.mxl" `
    "C:\Program Files\1cv8\srvinfo\reg_1541\<infobase_guid>\1Cv8Log\20251113144228.lgp" `
    "C:\Program Files\1cv8\srvinfo\reg_1541\<infobase_guid>\1Cv8Log\1Cv8.lgf"

# Сравнить данные GBIG
.\bin\compare.exe `
    "Тек конец ЖР_GBIG.mxl" `
    "C:\Program Files\1cv8\srvinfo\reg_1541\<infobase_guid>\1Cv8Log\20251113144228.lgp" `
    "C:\Program Files\1cv8\srvinfo\reg_1541\<infobase_guid>\1Cv8Log\1Cv8.lgf"
```

### Шаг 4: Анализ результатов

1. **Проверить количество записей:**
   - Если количество не совпадает, возможно, файлы не соответствуют друг другу

2. **Проверить совпадения:**
   - ✅ Match - парсер работает корректно для этих записей
   - ❌ Mismatch - нужно проверить, почему значения не совпадают

3. **Типичные проблемы:**
   - **Время:** Разница в часовых поясах (MXL в локальном времени, LGP может быть в UTC)
   - **Пользователь/Компьютер:** Не разрешены через LGF файл (нужно указать путь к .lgf)
   - **Метаданные:** Не разрешены через LGF файл
   - **Усеченные значения:** MXL содержит усеченные значения (проверяется по началу строки)

4. **Детальный анализ:**
   - Открыть `comparison_result.json` для детального сравнения всех полей

## Примечания

- **Усеченные значения:** MXL файлы могут содержать усеченные значения полей. Утилита `compare` проверяет совпадение по началу строки (HasPrefix), поэтому частичное совпадение считается корректным.

- **Разрешение значений:** Для правильного сравнения пользователей, компьютеров и метаданных необходимо указать путь к .lgf файлу. Без него парсер будет использовать только числовые ID, которые не совпадут с текстовыми значениями из MXL.

- **Формат времени:** MXL использует формат "DD.MM.YYYY HH:MM:SS" в локальном времени. Парсер конвертирует время из формата "YYYYMMDDHHMMSS" в UTC. Утилита учитывает возможную разницу в часовых поясах.

## Примеры использования

### Быстрая проверка парсера

```powershell
# 1. Извлечь данные
.\bin\extract_mxl.exe "Тек конец ЖР_DSSL.mxl"

# 2. Найти последний .lgp файл
$lgpFile = Get-ChildItem "C:\Program Files\1cv8\srvinfo\reg_*\*\1Cv8Log\*.lgp" | 
    Sort-Object LastWriteTime -Descending | 
    Select-Object -First 1 -ExpandProperty FullName

# 3. Найти соответствующий .lgf файл
$lgfFile = Join-Path (Split-Path $lgpFile) "1Cv8.lgf"

# 4. Сравнить
.\bin\compare.exe "Тек конец ЖР_DSSL.mxl" $lgpFile $lgfFile
```

### Автоматическая проверка всех файлов

```powershell
$mxlFiles = @("Тек конец ЖР_DSSL.mxl", "Тек конец ЖР_GBIG.mxl")
$basePath = "C:\Program Files\1cv8\srvinfo"

foreach ($mxl in $mxlFiles) {
    Write-Host "Processing $mxl..."
    
    # Извлечь данные
    .\bin\extract_mxl.exe $mxl
    
    # Найти соответствующие .lgp файлы (нужно настроить под вашу структуру)
    $lgpFiles = Get-ChildItem "$basePath\reg_*\*\1Cv8Log\*.lgp" | 
        Sort-Object LastWriteTime -Descending | 
        Select-Object -First 1
    
    foreach ($lgp in $lgpFiles) {
        $lgf = Join-Path $lgp.DirectoryName "1Cv8.lgf"
        if (Test-Path $lgf) {
            Write-Host "Comparing with $($lgp.Name)..."
            .\bin\compare.exe $mxl $lgp.FullName $lgf
        }
    }
}
```

---

**См. также:**
- [Спецификация парсера](log-service.spec.md)
- [Руководство по сравнению данных](comparison-guide.md)

