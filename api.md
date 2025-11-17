
# API для работы с платежами

Этот документ описывает, как правильно работать с партиционированной таблицей `payments` через API.

## Основные принципы

### 1. Обязательные поля при создании платежа

Внешний сервис должен отправить:

- **pay_date** – дата платежа (опционально, если используется UUID v7)
- **reference_number** – уникальный референс от провайдера
- **service_name** – название сервиса
- **amount** – сумма платежа
- **currency_code** – валюта (USD, UZS, EUR и т.д.)

### 2. UUID v7 и автоматическое извлечение даты

Если внешний сервис использует **UUID v7**, поле `pay_date` можно не передавать – база автоматически извлечёт дату из UUID:

```sql
-- UUID v7 содержит временную метку внутри себя
-- Функция uuid_extract_timestamp() извлекает её автоматически
```

**Преимущество:** меньше полей в запросе, меньше ошибок с несовпадением дат.

## Примеры запросов API

### Создание платежа (POST /payments)

#### Запрос

```json
POST /api/v1/payments
Content-Type: application/json

{
  "amount": 50000.00,
  "currency_code": "UZS",
  "pay_date": "2025-11-17",
  "service_name": "mobile",
  "reference_number": "MOB202511170001",
  "info": {
    "phone": "+998901234567",
    "operator": "Beeline",
    "plan": "Premium"
  }
}
```

#### SQL выполнение

```sql
INSERT INTO payments (pay_date, amount, currency_code, service_name, reference_number, info)
VALUES (
    '2025-11-17',
    50000.00,
    'UZS',
    'mobile',
    'MOB202511170001',
    '{"phone": "+998901234567", "operator": "Beeline", "plan": "Premium"}'::jsonb
)
RETURNING id, pay_date, status, created_at;
```

#### Ответ

```json
{
  "id": "019a90ee-8063-7de7-8c6a-da2f6518934a",
  "pay_date": "2025-11-17",
  "status": 1,
  "created_at": "2025-11-17T10:30:45.123456Z",
  "message": "Payment created successfully"
}
```

#### Вариант 2: Использование UUID v7 в reference_number (без pay_date)

```json
POST /api/v1/payments
Content-Type: application/json

{
  "amount": 50000.00,
  "currency_code": "UZS",
  "service_name": "mobile",
  "reference_number": "019a90ee-8063-7de7-8c6a-da2f6518934a",
  "info": {
    "phone": "+998901234567",
    "operator": "Beeline",
    "plan": "Premium"
  }
}
```

**Обратите внимание:** поле `pay_date` не передаётся, так как дата будет автоматически извлечена из UUID v7 в `reference_number`.

#### SQL выполнение

```sql
INSERT INTO payments (pay_date, amount, currency_code, service_name, reference_number, info)
VALUES (
    uuid_extract_timestamp('019a90ee-8063-7de7-8c6a-da2f6518934a')::DATE,
    50000.00,
    'UZS',
    'mobile',
    '019a90ee-8063-7de7-8c6a-da2f6518934a',
    '{"phone": "+998901234567", "operator": "Beeline", "plan": "Premium"}'::jsonb
)
RETURNING id, pay_date, status, created_at;
```

#### Ответ

```json
{
  "id": "019a90f1-2345-7abc-9def-abcdef123456",
  "pay_date": "2025-11-17",
  "status": 1,
  "created_at": "2025-11-17T10:30:45.123456Z",
  "message": "Payment created successfully"
}
```

## Важные замечания для API разработчиков

### 1. Всегда используйте pay_date в запросах

```sql
-- ❌ Плохо (медленно, сканирует все партиции)
SELECT * FROM payments WHERE id = '...';

-- ✅ Хорошо (быстро, сканирует только нужную партицию)
SELECT * FROM payments 
WHERE pay_date = uuid_extract_timestamp('...')::DATE 
  AND id = '...';
```

### 2. Валидация уникальности reference_number

В API документации укажите:

**Критически важно для корректной работы:**
 
1. **Уникальная комбинация:** `pay_date` + `reference_number` + `service_name` образуют уникальный ключ записи в системе.
 
2. **Запрещено дублирование:** Не может быть двух платежей с одинаковыми значениями всех трёх полей.
 
3. **Не генерируйте pay_date на стороне API!**  
    - Не генерировать pay_date (дату оплаты) "на лету" (т.е. в момент обработки), а необходимо получить его уникально вместе с reference_number (номером ссылки)
    - ❌ Плохо: `pay_date = time.Now()` при обработке запроса  
    - ✅ Правильно: получайте `pay_date` от клиента вместе с `reference_number`
 
4. **Почему это важно:**  
    - Если клиент отправит один и тот же `reference_number` дважды в разное время суток, но API будет генерировать `pay_date` сам, то появятся дубликаты платежей.
    - Клиент должен передавать согласованную пару: `reference_number` + `pay_date`, которая была сгенерирована в момент создания платёжного намерения.
 
 5. **Альтернатива (рекомендуется):**  
    Используйте **UUID v7** в качестве `reference_number` – он содержит временную метку, и `pay_date` можно извлечь автоматически через `uuid_extract_timestamp()`.
