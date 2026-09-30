# Сторонний код и происхождение

Лицензия проекта ([PolyForm Small Business 1.0.0](LICENSE)) распространяется на
собственный код 1c-log-checker и не перелицензирует сторонний код. Ниже —
заимствования и их лицензии.

## akpaevj/OneSTools.EventLog — MIT

- Источник: <https://github.com/akpaevj/OneSTools.EventLog>
- Что взято: логика разбора файлов журнала регистрации 1С (`.lgp`/`.lgf`),
  перенесённая с C# на Go с адаптацией: структура полей записи, разбор
  многострочных записей по балансу скобок, маппинг значений (уровни, приложения,
  типы событий), разбор транзакций из hex, чтение ссылочных объектов `.lgf`.
- Где: `internal/logreader/eventlog/lgp_parser.go`,
  `internal/logreader/eventlog/lgf_parser.go` (помечено комментариями
  «Based on OneSTools.EventLog …»).
- Эти фрагменты распространяются с сохранением уведомления MIT ниже; остальной
  код файлов и проекта — по лицензии проекта.

```
MIT License

Copyright (c) 2020 Akpaev Evgeny

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## hawkxtreme — только идея

Подход «парсинг файлов журнала + ClickHouse + Grafana» и выбор стека.
Код не заимствовался.

## Зависимости

Go-модули из `go.mod` (ClickHouse client, OpenTelemetry, Prometheus client,
zerolog, uuid, bbolt, yaml.v3 и их транзитивные зависимости) подключаются как
зависимости сборки и распространяются по своим лицензиям (MIT / Apache-2.0 /
BSD). В репозиторий они не вендорятся.
