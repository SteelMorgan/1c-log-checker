# Получение GUIDов кластера и информационной базы

Для корректной идентификации логов необходимо знать GUID кластера и информационной базы 1С:Предприятие.

---

## Метод 1: Использование `rac.exe`

`rac.exe` — утилита администрирования кластера 1С:Предприятие.

### Расположение

```
C:\Program Files\1cv8\<version>\bin\rac.exe
```

Например:
```
C:\Program Files\1cv8\8.3.24.1596\bin\rac.exe
```

### Получение GUID кластера

```powershell
# Подключение к серверу кластера
rac.exe cluster list

# Вывод (пример):
# cluster : af4fcd7c-0a86-11e7-8e5a-00155d000b0b
# host    : localhost
# port    : 1541
# name    : Local cluster
```

GUID кластера: `af4fcd7c-0a86-11e7-8e5a-00155d000b0b`

### Получение GUID информационной базы

```powershell
# Список информационных баз в кластере
rac.exe infobase summary list --cluster=af4fcd7c-0a86-11e7-8e5a-00155d000b0b

# Вывод (пример):
# infobase        : b8d1c34e-5f2e-11e9-80e4-00155d000c0a
# name            : erp_production
# dbms            : MSSQLServer
# db-server       : sql-server
# db-name         : erp_prod
# locale          : ru_RU
# date-offset     : 0
# scheduled-jobs-deny : off
```

GUID информационной базы: `b8d1c34e-5f2e-11e9-80e4-00155d000c0a`

---

## Метод 2: Просмотр структуры каталогов

Для серверных баз 1С логи располагаются в:

```
C:\Program Files\1cv8\srvinfo\reg_<port>\<cluster_guid>\<infobase_guid>\
```

Пример:
```
C:\Program Files\1cv8\srvinfo\reg_1541\
  └── af4fcd7c-0a86-11e7-8e5a-00155d000b0b\  ← GUID кластера
      └── b8d1c34e-5f2e-11e9-80e4-00155d000c0a\  ← GUID информационной базы
          └── 1Cv8Log\
              ├── 1Cv8.lgf
              └── *.lgp
```

Имена каталогов **и есть GUIDы**.

---

## Метод 3: Внешняя обработка 1С (TODO)

> Этот метод находится в разработке.

Планируется создать внешнюю обработку `.epf`, которая при запуске из 1С:Предприятие выведет GUIDы текущего кластера и информационной базы.

**Преимущества:**
- Не требует знания командной строки
- Работает для файловых баз
- Простой интерфейс

---

## Использование GUIDов в проекте

После получения GUIDов добавьте их в файл `cluster_map.yaml` в **корень проекта**. Образец находится в `configs/cluster_map.yaml`.

**Важно:** Файл `cluster_map.yaml` должен быть помещен в корень каждого проекта. Это необходимо, чтобы агент знал GUID базы/кластера, с которыми работает в проекте, и мог обращаться за логами конкретной базы.

```yaml
clusters:
  "af4fcd7c-0a86-11e7-8e5a-00155d000b0b":
    name: "Production Cluster"
    notes: "Main server cluster"

infobases:
  "b8d1c34e-5f2e-11e9-80e4-00155d000c0a":
    name: "ERP Production"
    cluster_guid: "af4fcd7c-0a86-11e7-8e5a-00155d000b0b"
    notes: "Main ERP database"
```

Это позволит парсеру и MCP-серверу преобразовывать GUIDы в человекочитаемые имена.

---

## Troubleshooting

### `rac.exe` не найден

Убедитесь, что путь к `rac.exe` добавлен в PATH, или используйте полный путь:

```powershell
& "C:\Program Files\1cv8\8.3.24.1596\bin\rac.exe" cluster list
```

### Ошибка подключения к кластеру

Проверьте, что сервер 1С:Предприятие запущен:

```powershell
Get-Service -Name "1C:Enterprise 8.3 Server Agent*"
```

Если служба остановлена:

```powershell
Start-Service -Name "1C:Enterprise 8.3 Server Agent (x86-64)"
```

### GUID не найден в каталогах

Для файловых баз логи располагаются в каталоге самой базы:

```
<путь к базе>\1Cv8Log\
```

GUID можно получить только через внешнюю обработку или смотреть в файле `.v8i` (секция `ID`).

---

**Следующие шаги:** После настройки `cluster_map.yaml` перезапустите сервисы для применения изменений.

