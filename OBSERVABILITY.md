# Observability для Link Tracker

## Метрики

### Scrapper-сервис

| Метрика | Тип | Лейблы | Описание |
|---------|-----|--------|----------|
| `links_on_track_total` | Gauge | `tracked_source` | Количество отслеживаемых ссылок по доменам (github.com, stackoverflow.com и др.) |
| `request_duration_ms_total` | Histogram | `scope`, `scope_type` | Длительность операций: БД (`database`), кэш (`cache`), внешние источники (`external_source`) |
| `api_requests_total` | Counter | `source` | Количество запросов к API Scrapper (источник: `telegram_bot`, `web_client`, `curl`, ...) |
| `http_requests_total` | Counter | `method`, `endpoint`, `status` | RED‑метрика: количество HTTP-запросов |
| `http_request_duration_seconds` | Histogram | `method`, `endpoint` | RED‑метрика: длительность запросов |
| `http_requests_in_flight` | Gauge | – | Текущее число обрабатываемых запросов |
| `memory_usage_bytes` | Gauge | – | Потребление оперативной памяти (в байтах) |

### Bot-сервис

| Метрика | Тип | Лейблы | Описание |
|---------|-----|--------|----------|
| `command_requests_total` | Counter | `command` | Количество обработанных команд (например, `/start`, `/list`, `/track`) |
| `command_duration_ms_total` | Histogram | `scope`, `scope_type` | Длительность выполнения команды (`scope="command"`, `scope_type` – имя команды) и вызовов Scrapper API (`scope="scrapper_sync_api"`) |
| `sent_notification_total` | Counter | – | Количество отправленных уведомлений (через Telegram) |
| `http_requests_total` | Counter | `method`, `endpoint`, `status` | RED‑метрика для HTTP‑эндпоинта `/updates` (если используется) |
| `http_request_duration_seconds` | Histogram | `method`, `endpoint` | RED‑метрика длительности |
| `http_requests_in_flight` | Gauge | – | Текущее число обрабатываемых запросов |
| `memory_usage_bytes` | Gauge | – | Потребление оперативной памяти |
