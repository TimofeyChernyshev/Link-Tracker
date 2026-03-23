# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

# Запуск бота в Telegram

1. Откройте в Telegram @BotFather
2. Выполните /newbot
3. Скопируйте выданный токен в `.env.bot` переменную `TELEGRAM_TOKEN` и запустите программу
4. Если запускаете через docker, то в `.env` также установите переменную `TELEGRAM_TOKEN` и запускайте через `docker-compose up -d --build`
