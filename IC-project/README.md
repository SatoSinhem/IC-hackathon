# IC Tender — starter repository

Стартовый монорепозиторий: `apps/web` — Next.js + TypeScript; `apps/api` — Go REST API; PostgreSQL — база данных; `apps/api/migrations` — SQL-миграции.

> Это каркас проекта, не готовая production-система. Сейчас есть минимальный Go API (`health`/`ready`), стартовый интерфейс и начальная схема БД. Авторизация, роли, тендерные сценарии, фоновая задача и импорт данных должны быть реализованы и протестированы следующими этапами.

## Требования
- Go 1.23+
- Node.js 20+ и npm
- Docker Desktop
- PostgreSQL 16+ (или Docker Compose)
- `migrate` CLI из golang-migrate для SQL-миграций

## Запуск
Нужны: Docker Desktop, Go, Node.js. Каждый шаг ниже выполняется в своём окне терминала, где указано.

### 1. Файлы окружения (один раз)
```powershell
Copy-Item .env.example .env
Copy-Item apps\web\.env.local.example apps\web\.env.local
Copy-Item apps\api\.env.example apps\api\.env
```
В корневом `.env` задай `POSTGRES_PASSWORD`. Без него `docker compose` не запустится.

### 2. База данных
```powershell
docker compose up -d db
```

### 3. Переменные для API и команд (в каждом новом окне заново)
`go run` сам `.env` не читает, поэтому переменную нужно задать в окне:
```powershell
$env:DATABASE_URL="postgres://ic_tender:ПАРОЛЬ@localhost:5432/ic_tender?sslmode=disable"
```

### 4. Миграции (один раз, версия схемы должна стать 3)
Если установлена утилита `migrate`:
```powershell
migrate -path apps\api\migrations -database "$env:DATABASE_URL" up
```
Если её нет, можно запустить через Docker (в адресе вместо `localhost` нужен `host.docker.internal`):
```powershell
docker run --rm -v "${PWD}\apps\api\migrations:/migrations" migrate/migrate -path=/migrations -database "postgres://ic_tender:ПАРОЛЬ@host.docker.internal:5432/ic_tender?sslmode=disable" up
```
Проверка версии:
```powershell
docker exec -it ic-tender-db psql -U ic_tender -d ic_tender -c "select * from schema_migrations"
```

### 5. Первый администратор (один раз, папка apps\api)
```powershell
cd apps\api
go run ./cmd/createuser

## Переменные окружения API
- `DATABASE_URL` — обязательна, значения по умолчанию нет.
- `WEB_ORIGIN` — origin фронтенда для CORS (`http://localhost:3000` в разработке).
- `APP_ENV` — `development` (по умолчанию) или `production`. В `production` API не запустится без `sslmode=require|verify-ca|verify-full`, без https в `WEB_ORIGIN` и без `SESSION_SECRET` и `INTERNAL_JOB_SECRET` (от 32 символов, значения разные).
- Порт PostgreSQL в `docker-compose.yml` привязан к `127.0.0.1`; пароль берётся из `.env`, без него Compose не запустится.
- `TRUST_FORWARDED_FOR` — `true`, если API доступен только через прокси Next.js (он добавляет адрес клиента). При `false` все пользователи за прокси выглядят одним адресом, и лимит попыток входа по адресу (20 за 15 минут) сработает для всех сразу. Включайте его, только когда API недоступен напрямую: иначе заголовок можно подделать.

## Авторизация
- Браузер общается с одним origin: Next.js проксирует `/api/*` на Go API (`API_INTERNAL_URL` в `apps/web/.env.local`, по умолчанию `http://localhost:8080`; старая `NEXT_PUBLIC_API_URL` больше не нужна). После изменения `next.config.ts` перезапустите dev-сервер.
- Сессия — cookie `ic_session` (`__Host-ic_session` в production): HttpOnly, SameSite=Lax. В БД хранится только HMAC-хеш токена. Срок: 2 ч бездействия, 7 дней абсолютно.
- Пароли — Argon2id, от 12 до 128 символов. После 5 неудачных попыток входа в одном аккаунте вход блокируется на 15 минут; счётчик хранится в памяти одного экземпляра API и сбрасывается при перезапуске. Обратная сторона: злоумышленник может блокировать чужую учётную запись, повторяя неверный пароль.
- Публичной регистрации нет. Первого администратора создаёт команда (пароль вводится скрыто или берётся из `CREATEUSER_PASSWORD`, в репозитории паролей нет; при первом входе потребуется смена пароля):
  ```bash
  set -a; source .env; set +a
  cd apps/api && go run ./cmd/createuser -email admin@example.com -role SUPER_ADMIN
  # SUBCONTRACTOR: -role SUBCONTRACTOR -subcontractor-id <id>
  ```
- Тесты на реальной БД запускаются только с `TEST_DATABASE_URL`, указывающим на отдельную базу с `test` в имени и применёнными миграциями (журнал аудита неизменяем, тестовые записи нельзя удалить). Без этой переменной эти тесты пропускаются.

## Интерфейс
- Тема чёрно-зелёная (CSS-переменные в `apps/web/src/app/globals.css`, цвета Tailwind в `tailwind.config.ts`). Компоненты `src/components/ui` написаны в стиле shadcn/ui на Tailwind и `class-variance-authority`.
- Все страницы закрыты проверкой сессии на сервере (`src/lib/server-session.ts`): каждая страница проверяет доступ сама, потому что layout не выполняется при переходах внутри приложения. `src/middleware.ts` лишь отсекает запросы без cookie. Скрытие пунктов меню — удобство, а не защита: права проверяет API.
- Разделы и права описаны одним списком в `src/lib/roles.ts`. Разделы без API показывают «Не подключено» и перечень планируемого, без вымышленных данных и неработающих кнопок.
- Юнит-тесты: `npm test` (Vitest). Тесты интерфейса: `npm run test:e2e` (Playwright). Им нужны запущенные API и фронтенд на одной БД и пять пользователей `admin@e2e.test`, `junior@e2e.test`, `finance@e2e.test` (с разрешением `VIEW_FINANCE`), `partner@e2e.test` (SUBCONTRACTOR), `newbie@e2e.test` (`must_change_password = true`; остальным флаг снят) с общим паролем в `E2E_PASSWORD`. Адрес фронтенда — `E2E_BASE_URL`. Тесты меняют пароль `newbie`, поэтому используйте отдельную одноразовую БД. Браузер: `npx playwright install chromium`.

См. `docs/implementation-plan.md` для следующего плана работ.

## admin@example.com password: 12 one - senior admin
## junior@example.com password: zxcqwerty123 - junior admin
## sub@example.com password: 123123123123 - our partner

