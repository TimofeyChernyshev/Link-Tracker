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

# Документы и скрины
## Результаты нагрузочных тестов и их анализа

- [docs/benchmark_results.md](docs/benchmark_results.md)

## Инфа по метрикам

- [json dashboard'a из графаны](docs/dashboard.json)
- [метрики в дашборде](docs/metrics.png)

# Обмен сообщениями между сервисами

`Scrapper` отправляет сообщения в Kafka (топик: `link.raw-updates`)

`Agent` читает из `link.raw-updates` и отправляет в `link.processed-updates` или, в случае ошибки, в `link.raw-updates-dlq`

`Bot` читает из `link.processed-updates`, в случае ошибки отправляет в `link.processed-updates-dlq`

При отсутствии возможности отправить в Kafka `Scrapper` отправляет сообщение напрямую в `Bot` по HTTP

# Как запускать

``` bash
docker-compose up --build
```

Создается кластер Kafka и кластер ValKey

## Переменные окружения

### Общие

#### Kafka
- `KAFKA_BROKERS` — список брокеров Kafka через запятую (обязательный, задаётся в `.env`)

| Общая переменная (`.env`) | Переменная в контейнере | Сервис | Назначение |
|---|---|---|---|
| `KAFKA_RAW_UPDATES_TOPIC` | `SCRAPPER_NOTIFIER_TOPIC` | Scrapper | Топик для отправки сырых обновлений |
| `KAFKA_RAW_UPDATES_TOPIC` | `AGENT_KAFKA_CONSUMER_TOPIC` | Agent | Топик для потребления сырых обновлений |
| `KAFKA_RAW_UPDATES_DLQ_TOPIC` | `AGENT_KAFKA_DLQ_TOPIC` | Agent | DLQ-топик сырых обновлений |
| `KAFKA_PROCESSED_UPDATES_TOPIC` | `AGENT_KAFKA_NOTIFIER_TOPIC` | Agent | Топик для отправки обработанных обновлений |
| `KAFKA_PROCESSED_UPDATES_TOPIC` | `BOT_KAFKA_CONSUMER_TOPIC` | Bot | Топик для потребления обработанных обновлений |
| `KAFKA_PROCESSED_UPDATES_DLQ_TOPIC` | `BOT_KAFKA_DLQ_TOPIC` | Bot | DLQ-топик обработанных обновлений |

**Примечание:** если сервисы запускаются без docker-compose, переменные с префиксами `SCRAPPER_`, `BOT_KAFKA_`, `AGENT_KAFKA_` нужно задавать напрямую.

#### Rate Limiter
Возможные префиксы: `SCRAPPER_RATE_LIMITER_`, `BOT_RATE_LIMITER_`

- `{PREFIX}_RPS` — количество запросов в секунду (по умолчанию: `50`)
- `{PREFIX}_BURST` — размер burst-буфера (по умолчанию: `100`)

#### Retry Config
Возможные префиксы: `BASIC_` - из Scrapper в Bot, `GITHUB_` - для скраппинга Github ссылок, `STACK_` - для скраппинга  StackOverflow ссылок, `SCRAPPER_` - из Bot в Scrapper

- `{PREFIX}_RETRY_MAX_ATTEMPTS` — максимальное количество попыток (по умолчанию: `3`)
- `{PREFIX}_RETRY_INITIAL_DELAY` — начальная задержка между попытками (по умолчанию: `100ms`)
- `{PREFIX}_RETRY_MAX_DELAY` — максимальная задержка между попытками (по умолчанию: `1s`)
- `{PREFIX}_RETRY_BACKOFF_FACTOR` — множитель backoff (по умолчанию: `1.0`)
- `{PREFIX}_RETRYABLE_HTTP_STATUSES` — HTTP-статусы для повторных попыток (по умолчанию: `500,502,503,504`)

#### Circuit Breaker Config
Возможные префиксы: `BASIC_CB_` - из Scrapper в Bot, `GITHUB_CB_` - для скраппинга Github ссылок, `STACK_CB_` - для скраппинга  StackOverflow ссылок, `SCRAPPER_CB_` - из Bot в Scrapper

- `{PREFIX}_MAX_REQUESTS` — максимальное количество запросов (по умолчанию: `5`)
- `{PREFIX}_INTERVAL` — интервал сброса счётчиков (по умолчанию: `0s`)
- `{PREFIX}_TIMEOUT` — таймаут выполнения запроса (по умолчанию: `1s`)
- `{PREFIX}_FAILURE_THRESHOLD` — порог ошибок для размыкания (по умолчанию: `5`)
- `{PREFIX}_SUCCESS_THRESHOLD` — порог успешных запросов для замыкания (по умолчанию: `2`)
- `{PREFIX}_FAILURE_RATE_THRESHOLD` — порог частоты ошибок в % (по умолчанию: `50`)
- `{PREFIX}_MINIMUM_CALLS` — минимальное количество вызовов для оценки (по умолчанию: `10`)
- `{PREFIX}_WINDOW_SIZE` — размер скользящего окна (по умолчанию: `10`)
- `{PREFIX}_PERMITTED_CALLS` — разрешённые вызовы в half-open (по умолчанию: `5`)
- `{PREFIX}_WAIT_DURATION` — время ожидания в open состоянии (по умолчанию: `1s`)

#### Kafka Consumer Config
Возможные префиксы: `BOT_KAFKA_`, `AGENT_KAFKA_`

- `{PREFIX}CONSUMER_TOPIC` — топик для потребления (обязательный). В docker-compose маппится из общих `KAFKA_*_TOPIC`
- `{PREFIX}GROUP_ID` — ID группы потребителей (обязательный)
- `{PREFIX}SESSION_TIMEOUT` — таймаут сессии консьюмера (по умолчанию: `10s`)
- `{PREFIX}MIN_BYTES` — минимальное количество байт для fetch-запроса (по умолчанию: `1`)
- `{PREFIX}MAX_BYTES` — максимальное количество байт для fetch-запроса (по умолчанию: `10485760`)

#### Kafka DLQ Config
Возможные префиксы: `BOT_KAFKA_`, `AGENT_KAFKA_`

- `{PREFIX}DLQ_TOPIC` — DLQ-топик (обязательный). В docker-compose маппится из общих `KAFKA_*_DLQ_TOPIC`
- `{PREFIX}MAX_RETRIES` — максимальное количество повторных попыток для DLQ (по умолчанию: `3`)
- `{PREFIX}RETRY_DELAY` — задержка между попытками отправки в DLQ (по умолчанию: `1s`)
- `{PREFIX}DLQ_BATCH_SIZE` — размер батча для DLQ (по умолчанию: `1`)
- `{PREFIX}DLQ_BATCH_TIMEOUT` — таймаут батча для DLQ (по умолчанию: `1s`)

#### Kafka Notifier Config
Возможные префиксы: `SCRAPPER_`, `AGENT_KAFKA_`

- `{PREFIX}NOTIFIER_TOPIC` — топик для отправки (обязательный). В docker-compose маппится из общих `KAFKA_*_TOPIC`
- `{PREFIX}NOTIFIER_COMPRESSION` — тип сжатия сообщений (по умолчанию: `snappy`)
- `{PREFIX}NOTIFIER_BATCH_SIZE` — размер батча для продюсера (по умолчанию: `100`)
- `{PREFIX}NOTIFIER_BATCH_TIMEOUT` — таймаут батча для продюсера (по умолчанию: `10ms`)
- `{PREFIX}NOTIFIER_REQUIRED_ACKS` — уровень подтверждений (`-1` = all) (по умолчанию: `-1`)

### Инфраструктура

#### Kafka
- `KAFKA_PROCESSED_UPDATES_TOPIC` — топик обработанных обновлений (по умолчанию: `link.processed-updates`)
- `KAFKA_PROCESSED_UPDATES_DLQ_TOPIC` — DLQ-топик обработанных обновлений (по умолчанию: `link.processed-updates-dlq`)
- `KAFKA_RAW_UPDATES_TOPIC` — топик сырых обновлений (по умолчанию: `link.raw-updates`)
- `KAFKA_RAW_UPDATES_DLQ_TOPIC` — DLQ-топик сырых обновлений (по умолчанию: `link.raw-updates-dlq`)

#### Valkey
- `VALKEY_NODE_1_PORT` — порт Valkey node 1 (по умолчанию: `6379`)
- `VALKEY_NODE_1_BUS_PORT` — bus-порт Valkey node 1 (по умолчанию: `16379`)
- `VALKEY_NODE_2_PORT` — порт Valkey node 2 (по умолчанию: `6380`)
- `VALKEY_NODE_2_BUS_PORT` — bus-порт Valkey node 2 (по умолчанию: `16380`)
- `VALKEY_NODE_3_PORT` — порт Valkey node 3 (по умолчанию: `6381`)
- `VALKEY_NODE_3_BUS_PORT` — bus-порт Valkey node 3 (по умолчанию: `16381`)
- `VALKEY_CLUSTER_ENABLED` — включение кластерного режима (по умолчанию: `yes`)
- `VALKEY_CLUSTER_NODE_TIMEOUT` — таймаут ноды кластера (по умолчанию: `5000`)
- `VALKEY_APPENDONLY` — включение AOF (по умолчанию: `yes`)
- `VALKEY_APPENDFSYNC` — стратегия fsync для AOF (по умолчанию: `everysec`)
- `VALKEY_MAXMEMORY` — максимальный объём памяти (по умолчанию: `256mb`)
- `VALKEY_MAXMEMORY_POLICY` — политика вытеснения ключей (по умолчанию: `allkeys-lru`)
- `VALKEY_NODE_1_CONF` — путь к файлу конфигурации node 1 (по умолчанию: `/data/nodes-1.conf`)
- `VALKEY_NODE_2_CONF` — путь к файлу конфигурации node 2 (по умолчанию: `/data/nodes-2.conf`)
- `VALKEY_NODE_3_CONF` — путь к файлу конфигурации node 3 (по умолчанию: `/data/nodes-3.conf`)
- `VALKEY_CLUSTER_REPLICAS` — количество реплик в кластере (по умолчанию: `0`)

### Scrapper

#### HTTP / Сервер
- `SCRAPPER_PORT` — порт, на котором работает Scrapper (обязательный)
- `SCRAPPER_METRIC_PORT` — порт для экспорта метрик
- `HTTP_DEFAULT_LIMIT` — количество элементов по умолчанию для пагинации (по умолчанию: `50`)
- `HTTP_MAX_LIMIT` — максимальное количество элементов для пагинации (по умолчанию: `100`)
- `SCRAPPER_SHUTDOWN_TIMEOUT` — таймаут graceful shutdown (по умолчанию: `30s`)

#### База данных
- `ACCESS_TYPE` — тип доступа к БД: `sql` или `orm` (обязательный)
- `DB_USER` — пользователь БД (обязательный)
- `DB_PASSWORD` — пароль БД (обязательный)
- `DB_HOST` — хост БД (обязательный)
- `DB_PORT` — порт БД (обязательный)
- `DB_NAME` — имя БД (обязательный)

#### Планировщик проверки ссылок
- `CHECK_INTERVAL` — интервал между проверками ссылок (по умолчанию: `60s`)
- `SCRAPPER_WORKER_COUNT` — количество воркеров для проверки (по умолчанию: `4`, min: 1, max: 20)
- `SCRAPPER_BATCH_SIZE` — размер батча для проверки (по умолчанию: `20`, min: 10, max: 1000)

#### Link Checker (GitHub & StackOverflow)
- `API_BATCH_SIZE` — размер батча для API-запросов (по умолчанию: `100`, min: 50, max: 500)
- `GITHUB_BASE_URL` — базовый URL GitHub API (обязательный)
- `STACK_BASE_URL` — базовый URL StackOverflow API (обязательный)
- `CHECKER_PREVIEW_LEN` — длина превью для результатов проверки (по умолчанию: `200`)

#### Valkey (кэш)
- `VALKEY_ADDRESSES` — список адресов Valkey через запятую (обязательный)
- `VALKEY_PASSWORD` — пароль для подключения к Valkey
- `VALKEY_TTL` — время жизни записей в кэше (по умолчанию: `300s`)
- `VALKEY_MAX_RETRIES` — максимальное количество повторных попыток (по умолчанию: `3`)
- `VALKEY_POOL_SIZE` — размер пула соединений (по умолчанию: `10`)
- `VALKEY_MIN_RETRY_BACKOFF` — минимальная задержка между повторными попытками (по умолчанию: `100ms`)
- `VALKEY_MAX_RETRY_BACKOFF` — максимальная задержка между повторными попытками (по умолчанию: `1s`)
- `VALKEY_PING_TIME` — интервал ping-запросов (по умолчанию: `5s`)
- `VALKEY_CLUSTER_MODE` — режим кластера (по умолчанию: `true`)
- `VALKEY_SCAN_COUNT` — количество ключей за одну итерацию SCAN (по умолчанию: `100`)

#### Rate Limiter (HTTP сервер Scrapper)
Префикс: `SCRAPPER_RATE_LIMITER_`
- См. общий шаблон Rate Limiter

#### Basic HTTP клиент
Префиксы:
- Retry: `BASIC_`
- Circuit Breaker: `BASIC_CB_`
- `BASIC_RATE_LIMIT` — лимит запросов (по умолчанию: `100`)
- `BASIC_TIMEOUT` — таймаут запросов (по умолчанию: `5s`)

#### GitHub HTTP клиент
Префиксы:
- Retry: `GITHUB_`
- Circuit Breaker: `GITHUB_CB_`
- `GITHUB_RATE_LIMIT` — лимит запросов (по умолчанию: `100`)
- `GITHUB_TIMEOUT` — таймаут запросов (по умолчанию: `5s`)

#### StackOverflow HTTP клиент
Префиксы:
- Retry: `STACK_`
- Circuit Breaker: `STACK_CB_`
- `STACK_RATE_LIMIT` — лимит запросов (по умолчанию: `100`)
- `STACK_TIMEOUT` — таймаут запросов (по умолчанию: `5s`)

#### Kafka Notifier (отправка сырых обновлений)
Префикс: `SCRAPPER_`
- `SCRAPPER_NOTIFIER_TOPIC` — топик для отправки сырых обновлений (обязательный). В docker-compose маппится из `KAFKA_RAW_UPDATES_TOPIC`
- Остальные переменные — см. общий шаблон Kafka Notifier Config с префиксом `SCRAPPER_`

#### Метрики
- `SCRAPPER_MEMORY_METRIC_TICK` — интервал сбора метрик памяти (по умолчанию: `30s`)

### Bot

#### Telegram
- `TELEGRAM_TOKEN` — токен Telegram бота (обязательный)
- `TELEGRAM_API_URL` — URL Telegram API

#### HTTP / Сервер
- `BOT_PORT` — порт, на котором работает Bot (обязательный)
- `BOT_METRIC_PORT` — порт для экспорта метрик
- `BOT_SHUTDOWN_TIMEOUT` — таймаут graceful shutdown (по умолчанию: `30s`)

#### Scrapper клиент
Префиксы:
- Retry: `SCRAPPER_`
- Circuit Breaker: `SCRAPPER_CB_`
- `SCRAPPER_BASE_URL` — базовый URL Scrapper (обязательный)
- `SCRAPPER_RATE_LIMIT` — лимит запросов к Scrapper (по умолчанию: `100`)
- `SCRAPPER_TIMEOUT` — таймаут запросов к Scrapper (по умолчанию: `5s`)

#### Таймауты обработчиков
- `TIMEOUT_CHECK_LINK` — таймаут проверки ссылки (по умолчанию: `10s`)
- `TIMEOUT_SAVE_LINK` — таймаут сохранения ссылки (по умолчанию: `5s`)
- `TIMEOUT_START_HANDLER` — таймаут команды /start (по умолчанию: `5s`)
- `TIMEOUT_UNTRACK_HANDLER` — таймаут команды /untrack (по умолчанию: `5s`)
- `TIMEOUT_LIST_HANDLER` — таймаут команды /list (по умолчанию: `5s`)

#### Воркеры и буферы
- `BOT_WORKER_COUNT` — количество воркеров обработки (по умолчанию: `8`)
- `BOT_SENDER_COUNT` — количество отправителей сообщений (по умолчанию: `4`)
- `BOT_JOBS_BUFFER_SIZE` — размер буфера задач (по умолчанию: `100`)
- `BOT_OUTGOING_BUFFER_SIZE` — размер буфера исходящих сообщений (по умолчанию: `100`)

#### Rate Limiter (HTTP сервер Bot)
Префикс: `BOT_RATE_LIMITER_`
- См. общий шаблон Rate Limiter

#### Kafka Consumer (получение обработанных обновлений)
Префикс: `BOT_KAFKA_`
- `BOT_KAFKA_CONSUMER_TOPIC` — топик для потребления обработанных обновлений (обязательный). В docker-compose маппится из `KAFKA_PROCESSED_UPDATES_TOPIC`
- Остальные переменные — см. общий шаблон Kafka Consumer Config с префиксом `BOT_KAFKA_`

#### Kafka DLQ
Префикс: `BOT_KAFKA_`
- `BOT_KAFKA_DLQ_TOPIC` — DLQ-топик для обработанных обновлений (обязательный). В docker-compose маппится из `KAFKA_PROCESSED_UPDATES_DLQ_TOPIC`
- Остальные переменные — см. общий шаблон Kafka DLQ Config с префиксом `BOT_KAFKA_`

#### Метрики
- `BOT_MEMORY_METRIC_TICK` — интервал сбора метрик памяти (по умолчанию: `30s`)

### Agent

#### Общие
- `AGENT_SHUTDOWN_TIMEOUT` — таймаут graceful shutdown (по умолчанию: `30s`)
- `AGENT_SEND_UPDATE_TIMEOUT` — таймаут отправки обновления (по умолчанию: `30s`)

#### Kafka Consumer (получение сырых обновлений)
Префикс: `AGENT_KAFKA_`
- `AGENT_KAFKA_CONSUMER_TOPIC` — топик для потребления сырых обновлений (обязательный). В docker-compose маппится из `KAFKA_RAW_UPDATES_TOPIC`
- Остальные переменные — см. общий шаблон Kafka Consumer Config с префиксом `AGENT_KAFKA_`

#### Kafka DLQ
Префикс: `AGENT_KAFKA_`
- `AGENT_KAFKA_DLQ_TOPIC` — DLQ-топик для сырых обновлений (обязательный). В docker-compose маппится из `KAFKA_RAW_UPDATES_DLQ_TOPIC`
- Остальные переменные — см. общий шаблон Kafka DLQ Config с префиксом `AGENT_KAFKA_`

#### Kafka Notifier (отправка обработанных обновлений)
Префикс: `AGENT_KAFKA_`
- `AGENT_KAFKA_NOTIFIER_TOPIC` — топик для отправки обработанных обновлений (обязательный). В docker-compose маппится из `KAFKA_PROCESSED_UPDATES_TOPIC`
- Остальные переменные — см. общий шаблон Kafka Notifier Config с префиксом `AGENT_KAFKA_`

#### Фильтрация
- `FILTER_STOP_WORDS` — список стоп-слов для фильтрации (через запятую)
- `FILTER_EXCLUDED_AUTHORS` — список исключаемых авторов (через запятую)
- `FILTER_MIN_LENGTH` — минимальная длина сообщения (по умолчанию: `20`)

#### Суммаризация
- `SUMMARIZATION_THRESHOLD` — порог длины текста для включения суммаризации (по умолчанию: `500`)

#### Приоритизация
- `PRIORITIZATION_HIGH_KEYWORDS` — ключевые слова для высокого приоритета (через запятую)
- `PRIORITIZATION_LOW_KEYWORDS` — ключевые слова для низкого приоритета (через запятую)

#### Группировка
- `GROUPING_WINDOW` — окно группировки обновлений (по умолчанию: `30000ms`)