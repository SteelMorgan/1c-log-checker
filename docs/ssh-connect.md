# Подключение к серверу 1С по SSH

## Параметры подключения

| Параметр     | Значение                              |
|-------------|---------------------------------------|
| Хост        | `192.168.250.2`                       |
| Пользователь| `sandbox`                             |
| Ключ        | `.ssh/onec-infra/id_ed25519`          |

Все пути указаны относительно корня проекта.

## Подключение из рабочего каталога проекта

```bash
ssh -i .ssh/onec-infra/id_ed25519 -o StrictHostKeyChecking=no sandbox@192.168.250.2
```

## Выполнение команды без входа в сессию

```bash
ssh -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2 "команда"
```

## Примеры управления 1С

```bash
# Список процессов 1С
ssh -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2 "ps aux | grep ragent"

# Состояние сервиса
ssh -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2 "systemctl status srv1cv8-8.3.25.1257"

# Список информационных баз (через rac)
ssh -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2 "rac infobase summary list --cluster=<cluster-uuid>"

# Просмотр экспортированных логов
ssh -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2 "ls -la /opt/onec-export/srvinfo/"

# Копирование файла с сервера
scp -i .ssh/onec-infra/id_ed25519 sandbox@192.168.250.2:/путь/к/файлу ./локальный_путь
```

## Из Docker-контейнера

Парсер подключается автоматически через `entrypoint-parser.sh`. Конфигурация в `deploy/docker/.env`:

```env
ONEC_VM_IP=192.168.250.2
ONEC_VM_USER=sandbox
ONEC_SSH_KEY_PATH=repos/1C Framework/1c-log-checker/.ssh/onec-infra/id_ed25519
```
