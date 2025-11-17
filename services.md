# Партиционирование таблицы платёжных сервисов (payments)

Этот файл демонстрирует, как создать партиционированную таблицу для платежей через различные сервисы (коммунальные услуги, мобильная связь, интернет и т.д.) с помощью TimescaleDB.

## Структура таблицы

```sql
CREATE TABLE payments(
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    pay_date                    DATE DEFAULT CURRENT_DATE,
    id                          UUID  DEFAULT uuidv7(),
    amount                      NUMERIC(18, 2) NOT NULL,
    status                      SMALLINT DEFAULT 1,
    error_code                  SMALLINT,
    currency_code               CHAR(3) NOT NULL,
    service_name                TEXT NOT NULL,
    reference_number            TEXT NOT NULL,
    info                        JSONB,
    PRIMARY KEY (id, pay_date)
)
```

### Ключевые поля

- **id** – уникальный идентификатор платежа (UUID v7).
- **pay_date** – дата платежа, используется как **ключ партиционирования**.
- **amount** / **currency_code** – сумма и валюта платежа.
- **status** – статус платежа:
  - `1` – создан (created)
  - `2` – обрабатывается (processing)
  - `3` – завершён (completed)
  - `4` – ошибка (failed)
- **error_code** – код ошибки (если платёж не прошёл).
- **service_name** – название сервиса (например, `payroll`, `mobile`, `utilities`, `internet`).
- **reference_number** – уникальный референс платежа от провайдера.
- **info** – дополнительные данные в формате JSONB (гибкая структура для каждого типа сервиса).

### Настройки hypertable TimescaleDB

```sql
WITH (
    tsdb.hypertable,
    tsdb.partition_column = 'pay_date',
    tsdb.create_default_indexes = false,
    tsdb.segmentby        = 'service_name',
    tsdb.orderby          = 'pay_date DESC'
)
```

- **partition_column = 'pay_date'** – таблица разбивается на партиции по дате платежа.
- **segmentby = 'service_name'** – внутри партиций данные сегментируются по названию сервиса (оптимизация для запросов по конкретному сервису).
- **orderby = 'pay_date DESC'** – строки хранятся в порядке убывания даты (последние платежи находятся быстрее).
- **create_default_indexes = false** – отключаем автоматические индексы, создаём свои вручную.

### Индексы

```sql
CREATE UNIQUE INDEX idx_payment_service_ref_num ON payments(service_name, reference_number, pay_date);
```

Уникальный индекс на комбинацию `service_name + reference_number + pay_date` гарантирует, что **в рамках одного сервиса в один день** не будет дублирующихся референсов.

**Важно для API:**

- Этот constraint нужно документировать в API документации.
- При приёме платежа через API проверяйте уникальность `reference_number` для конкретного `service_name` в текущий день.
- Если платёж с таким референсом уже есть сегодня для этого сервиса → возвращайте ошибку (409 Conflict или идентификатор существующего платежа).

## Проверка дубликатов за больший период

```sql
-- Опционально: дополнительная проверка на дубликаты за больший период (не только сегодня)
SELECT EXISTS(
    select 1 
    from payments 
    where pay_date > '2025-10-01' 
      AND service_name = 'payroll' 
      AND reference_number = 'PAY123456'
);
```

**Зачем это нужно:**

Иногда нужно проверить уникальность референса не только за сегодня, но и за последний месяц/неделю:

- Защита от повторной обработки платежа, который был вчера/неделю назад.
- Можно запустить как фоновую задачу перед обработкой платежа.
- Или использовать в API для дополнительной валидации.

**Рекомендация:** если вам нужна строгая глобальная уникальность референсов (независимо от даты), лучше использовать отдельную таблицу-справочник с обычным UNIQUE индексом на `service_name + reference_number`.

## Примеры использования

### Вставка платежа

```sql
INSERT INTO payments (amount, currency_code, service_name, reference_number, info)
VALUES (
    150.50,
    'USD',
    'mobile',
    'MOB789012',
    '{"phone": "+998901234567", "operator": "Beeline"}'::jsonb
);
```

### Поиск платежей по сервису за день

```sql
SELECT * FROM payments
WHERE pay_date = '2025-11-17'
  AND service_name = 'payroll';
```

Благодаря `segmentby = 'service_name'` этот запрос будет очень быстрым.

### Поиск по референсу в конкретный день

```sql
SELECT * FROM payments
WHERE pay_date = '2025-11-17'
  AND service_name = 'utilities'
  AND reference_number = 'UTIL456789';
```

### Статистика по сервису за месяц

```sql
SELECT 
    service_name,
    COUNT(*) as total_payments,
    SUM(amount) as total_amount,
    COUNT(CASE WHEN status = 3 THEN 1 END) as completed,
    COUNT(CASE WHEN status = 4 THEN 1 END) as failed
FROM payments
WHERE pay_date BETWEEN '2025-11-01' AND '2025-11-30'
GROUP BY service_name;
```

### Обновление статуса платежа

```sql
UPDATE payments
SET status = 3, updated_at = CURRENT_TIMESTAMP
WHERE pay_date = uuid_extract_timestamp('019a90ee-8063-7de7-8c6a-da2f6518934a')::DATE
  AND id = '019a90ee-8063-7de7-8c6a-da2f6518934a';
```

**Критически важно:** всегда указывайте `pay_date` в WHERE, иначе будут сканироваться все партиции.

## Рекомендации

1. **Всегда фильтруйте по `pay_date`** – это ключ партиционирования, без него запросы медленные.
2. **Используйте `service_name` в запросах** – благодаря `segmentby` это ускорит поиск.
3. **Документируйте constraint в API** – объясните клиентам, что `reference_number` должен быть уникален для каждого сервиса в рамках дня.
4. **JSONB для гибкости** – поле `info` позволяет хранить специфичные для каждого сервиса данные без изменения схемы таблицы.
5. **Для батчевых вставок используйте `COPY FROM`** – см. `tips.md` для примера высокопроизводительной вставки.
6. **Настройте columnstore для старых данных:**

```sql
-- Автоматически сжимать данные старше 6 месяцев
CALL add_columnstore_policy('payments', after => INTERVAL '180d');
```

## Работа с JSONB полем `info`

### Запрос с фильтром по JSONB

```sql
-- Найти все платежи Beeline за сегодня
SELECT * FROM payments
WHERE pay_date = CURRENT_DATE
  AND service_name = 'mobile'
  AND info->>'operator' = 'Beeline';
```

### Индекс на JSONB поле (опционально)

Если часто фильтруете по полю внутри `info`, создайте GIN индекс:

```sql
CREATE INDEX idx_payments_info_gin ON payments USING GIN (info);
```

Или индекс на конкретное поле:

```sql
CREATE INDEX idx_payments_operator ON payments ((info->>'operator'));
```

Это ускорит запросы по данным внутри JSONB.
