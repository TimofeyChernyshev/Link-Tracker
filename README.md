# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

# Запуск бота в Telegram

1. Откройте в Telegram @BotFather
2. Выполните /newbot
3. Скопируйте выданный токен в `.env.bot` переменную `TELEGRAM_TOKEN` и запустите программу
4. Если запускаете через docker, то в `.env` также установите переменную `TELEGRAM_TOKEN` и запускайте через `docker-compose up -d --build`

# Запуск интеграционных тестов

Перед запуском соберите образы через bot и scrapper с именами

- link-tracker-bot
- link-tracker-scrapper

# Результаты нагрузочных тестов и их анализа

[docs/benchmark_results.md](docs/benchmark_results.md)

# Обмен сообщениями между сервисами

`Scrapper` отправляет сообщения в Kafka (топик: `link.raw-updates`)

`Agent` читает из `link.raw-updates` и отправляет в `link.processed-updates` или, в случае ошибки, в `link.raw-updates-dlq`

`Bot` читает из `link.processed-updates`, в случае ошибки отправляет в `link.processed-updates-dlq`

При отсутствии возможности отправить в Kafka `Scrapper` отправляет сообщение напрямую в `Bot` по HTTP

