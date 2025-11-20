# Партиционирование таблицы транзакций (payment_orders)

Этот файл демонстрирует, как создать партиционированную таблицу для платёжных заказов с помощью TimescaleDB и правильно с ней работать.

## Структура таблицы

```sql
CREATE TABLE payment_orders(
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    scheduled_execution_date    DATE,
    order_date                  DATE DEFAULT CURRENT_DATE,
    id                          UUID  DEFAULT uuidv7(),
    sender_account_id           BIGINT NOT NULL,
    beneficiary_account_id      BIGINT NOT NULL,
    amount                      NUMERIC(18, 2) NOT NULL,
    status                      SMALLINT DEFAULT 1,
    currency_code               CHAR(3) NOT NULL,
    reference_number            TEXT NOT NULL,
    payment_method              TEXT,
    PRIMARY KEY (id, order_date)
)
```

### Ключевые поля

- **id** – уникальный идентификатор заказа (UUID v7, содержит временную метку).
- **order_date** – дата заказа, используется как **ключ партиционирования**.
- **sender_account_id** / **beneficiary_account_id** – счета отправителя и получателя.
- **amount** / **currency_code** – сумма и валюта платежа.
- **status** – статус заказа (например, 1 = создан, 2 = обработан).
- **reference_number** – уникальный референс платежа (в пределах одного дня).

### Настройки hypertable TimescaleDB

```sql
WITH (
    tsdb.hypertable,
    tsdb.partition_column = 'order_date',
    tsdb.create_default_indexes = false,
    tsdb.segmentby        = 'sender_account_id',
    tsdb.orderby          = 'order_date DESC'
)
```


Когда вы создаете гипертаблицу с помощью CREATE TABLE ... WITH ..., столбец партиционирования по умолчанию автоматически становится первым столбцом с типом данных timestamp. Также TimescaleDB создает политику columnstore, которая автоматически преобразует ваши данные в columnstore через интервал, равный значению chunk_interval, определенному через compress_after в политике. Этот колоночный формат обеспечивает быстрое сканирование и агрегацию, оптимизируя производительность для аналитических нагрузок и значительно экономя дисковое пространство. При преобразовании в columnstore чанки гипертаблицы сжимаются до 98% и организуются для эффективных масштабных запросов.


- **partition_column = 'order_date'** – таблица разбивается на партиции по дате заказа.
- **segmentby = 'sender_account_id'** – внутри партиций данные дополнительно сегментируются по счёту отправителя (оптимизация для запросов по `sender_account_id`).
- **orderby = 'order_date DESC'** – строки внутри сегментов хранятся в порядке убывания даты (ускоряет запросы последних заказов).
- **create_default_indexes = false** – отключает автоматическое создание индексов (мы создаём свои вручную).

### Индексы

```sql
CREATE UNIQUE INDEX idx_payment_order_ref_num ON payment_orders(reference_number, order_date);
```

Уникальный индекс на `reference_number` и `order_date` гарантирует, что в пределах одного дня не будет дублей по референсу.

**Важно:** UNIQUE constraint работает только внутри одной партиции (т.е. в пределах одного дня). Если референс должен быть уникальным глобально, потребуется другой подход (например, отдельная таблица-справочник).

## Вставка данных

```sql
insert into payment_orders (sender_account_id, beneficiary_account_id, amount, currency_code, reference_number, payment_method, scheduled_execution_date) 
values
(1001, 2001, 1500.00, 'USD', 'REF123456', 'Wire Transfer', '2025-12-01'),
...
```

При вставке `order_date` автоматически заполняется текущей датой (если не указано явно). TimescaleDB автоматически направит строки в нужные партиции.

## Обновление и удаление

```sql
update payment_orders 
set status = 2, updated_at = CURRENT_TIMESTAMP 
where order_date = uuid_extract_timestamp('019a90ee-8063-7de7-8c6a-da2f6518934a')::DATE 
  and id = '019a90ee-8063-7de7-8c6a-da2f6518934a';
```

**Критически важно:** всегда указывайте `order_date` в условиях `WHERE` при обновлении или удалении записей. Без этого база будет сканировать **все партиции**, что очень медленно.

В примере выше используется функция `uuid_extract_timestamp()` для извлечения даты из UUID v7.

## Ограничение уникальности

```sql
-- Попытка вставить дублирующий reference_number в другой день пройдёт успешно:
insert into payment_orders (sender_account_id, beneficiary_account_id, amount, currency_code, reference_number, payment_method, scheduled_execution_date) 
values
(1006, 2006, 1800.00, 'USD', 'REF123456', 'Wire Transfer', '2025-12-06');
```

Поскольку `order_date` для этой записи будет `2025-12-06`, а UNIQUE индекс работает только в пределах партиции (одного дня), вставка пройдёт успешно, даже если `REF123456` уже существует в другой день.

## Правильные запросы (с фильтром по order_date)

### Поиск по референсу в конкретный день

```sql
select * from payment_orders 
where order_date = '2025-11-01' 
  and reference_number = 'REF123456';
```

Сканируется только одна партиция → быстро.

### Поиск по счёту отправителя за диапазон дат

```sql
select * from payment_orders 
where order_date > '2025-11-02' 
  and sender_account_id = 1002;
```

Благодаря `segmentby = 'sender_account_id'` запрос будет эффективно работать внутри нужных сегментов.

### Запрос за период с условием по сумме

```sql
select * from payment_orders 
where order_date BETWEEN '2025-11-01' AND '2025-11-30' 
  and amount > 1000.00;
```

Сканируются только партиции за ноябрь → намного быстрее, чем сканирование всей таблицы.

## Рекомендации

1. **Всегда фильтруйте по `order_date`** – это ключ партиционирования, без него запросы будут очень медленными.
2. **Используйте UUID v7** – он содержит временную метку, что позволяет извлекать дату без дополнительных полей.
3. **UNIQUE constraint работает только внутри партиции** – если нужна глобальная уникальность, храните референсы в отдельной таблице.
4. **Для батчевых вставок используйте `COPY FROM`** – аналогично логам (см. `tips.md`), это даст максимальную производительность.
5. **Настройте политику сжатия (columnstore)** – автоматически сжимайте старые данные, чтобы сэкономить место и ускорить аналитические запросы.

## Пример политики сжатия старых партиций

```sql
-- Автоматически сжимать данные старше 170 дней в columnstore формат
CALL add_columnstore_policy('payment_orders', after => INTERVAL '170d');
```

**Зачем это нужно:**

- **Экономия места** – columnstore формат сжимает данные в 5-10 раз эффективнее обычного row-based хранения.
- **Быстрее аналитика** – запросы с агрегацией (SUM, COUNT, AVG) на старых данных работают значительно быстрее.
- **Платежи остаются доступны** – в отличие от retention policy, данные не удаляются, а просто переводятся в более эффективный формат для хранения и чтения.

Старые платежи (старше ~6 месяцев) обычно не изменяются, только читаются для отчётов и аналитики, поэтому columnstore для них оптимален.


```sql

update payment_orders set amount = 57413.22  where order_date = '2025-09-13' and id = '019a9a89-894e-789d-a14c-d056dad9864e';
UPDATE 1
Time: 10.477 ms
update payment_orders set amount = 57413.22  where  id = '019a9a89-894e-789d-a14c-d056dad9864e';
UPDATE 1
Time: 56.182 ms

```