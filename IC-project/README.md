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
1. В корне: `cp .env.example .env`
2. В `apps/web`: `cp .env.local.example .env.local`
3. Запусти PostgreSQL: `docker compose up -d db`
4. Примени миграцию:
   ```bash
   set -a; source .env; set +a
   migrate -path apps/api/migrations -database "$DATABASE_URL" up
   ```
5. В первом терминале запусти API:
   ```bash
   set -a; source .env; set +a
cd apps/api && go mod tidy && go run ./cmd/api
   ```
6. Во втором терминале запусти frontend:
   ```bash
   cd apps/web && npm install && npm run dev
   ```
7. Открой `http://localhost:3000`. API: `http://localhost:8080/api/v1/health` и `/api/v1/ready`.

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

## Важно
- Победителя выбирает человек с нужными правами.
- Цену тендера задаёт IC Group; субподрядчик не предлагает встречную цену.
- Единственный автоматический триггер изменения суммы открытого тендера — приближение дедлайна.
- После выбора победителя автоматическое изменение суммы прекращается.
- Внутренняя маржа, рейтинги и штрафные начисления не показываются субподрядчику.
- Клиентских аккаунтов нет.
- В кабинете субподрядчика предусмотрены «Доступные тендеры», «Мои отклики», «Мои объекты», «Выполненные объекты».
- Истёкший контракт не считается завершённым без подтверждения администратора.
- Не используй демонстрационные пароли в production.

См. `docs/implementation-plan.md` для следующего плана работ.

## admin@example.com password: 12 one - senior admin
## junior@example.com password: zxcqwerty123 - junior admin
## sub@example.com password: 123123123123 - our partner