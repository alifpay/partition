
```sql


-- table about account details
CREATE TABLE accounts(
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    id              BIGSERIAL PRIMARY KEY,
    customer_id     BIGINT NOT NULL,
    currency_code   CHAR(3) NOT NULL,
    balance         NUMERIC(18, 2) NOT NULL,
    account_holder  TEXT NOT NULL,
    account_type    TEXT NOT NULL
);

-- customers table
CREATE TABLE customers(
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    birthdate DATE,
    id      BIGSERIAL PRIMARY KEY,
    name    TEXT NOT NULL,
    email   TEXT NOT NULL,
    phone   TEXT,
    address TEXT,
    passport TEXT
);

CREATE TABLE payment_orders2(
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    scheduled_execution_date    DATE,
    id                          UUID  DEFAULT uuidv7(),
    sender_account_id           BIGINT NOT NULL,
    beneficiary_account_id      BIGINT NOT NULL,
    amount                      NUMERIC(18, 2) NOT NULL,
    status                      SMALLINT DEFAULT 1,
    currency_code               CHAR(3) NOT NULL,
    reference_number            TEXT UNIQUE NOT NULL,
    payment_method              TEXT,
    PRIMARY KEY (id)
);
```

batch insert for payment_orders2 took   
Вставка 10000000 платежных ордеров завершена за 3m3.546634264s

batch insert for payment_orders took.   
Вставка 10000000 платежных ордеров завершена за 2m51.342653237s




```sql

select status,count(id) from payment_orders2 where uuid_extract_timestamp(id) >='2025-10-01' group by status;
 status |  count
--------+---------
      1 | 2001634
      2 | 1999808
      3 | 1998045
      4 | 1999974
      5 | 2000539
(5 rows)

Time: 4223.653 ms (00:04.224)


select status,count(id) from payment_orders where order_date >='2025-10-01' group by status;
 status | count
--------+--------
      3 | 269484
      5 | 269925
      4 | 269361
      2 | 269858
      1 | 270307
(5 rows)

Time: 160.596 ms


select status,count(id) from payment_orders2 where created_at >='2025-10-01' group by status;
 status | count
--------+--------
      1 | 269278
      2 | 269449
      3 | 269464
      4 | 269620
      5 | 269153
(5 rows)

Time: 743.644 ms

CREATE INDEX idx_payment_order2_date ON payment_orders2(created_at);

```

let's create index for created_at


after index create the same time

```sql

explain analyze select *  from payment_orders2 where created_at >='2025-10-01';

 Index Scan using idx_payment_order2_date on payment_orders2  (cost=0.43..208818.90 rows=1363067 width=95) (actual time=0.000..9329.642 rows=1346964.00 loops=1)
   Index Cond: (created_at >= '2025-10-01 00:00:00+00'::timestamp with time zone)
   Index Searches: 1
   Buffers: shared hit=1205784 read=144724
 Planning:
   Buffers: shared hit=29 read=6 dirtied=8
 Planning Time: 0.000 ms
 Execution Time: 9411.894 ms
(8 rows)

Time: 9411.894 ms (00:09.412)


```


in partition

```sql

explain analyze select *  from payment_orders where order_date >='2025-10-01';


Append  (cost=0.00..49535.94 rows=1349025 width=99) (actual time=0.000..203.625 rows=1348935.00 loops=1)
   Buffers: shared hit=24908 read=947
   ->  Seq Scan on _hyper_1_16_chunk  (cost=0.00..5561.80 rows=192064 width=99) (actual time=0.000..0.000 rows=192147.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3161
   ->  Index Scan using "20_20_payment_orders_pkey" on _hyper_1_20_chunk  (cost=0.42..4518.64 rows=27091 width=99) (actual time=0.000..89.274 rows=27074.00 loops=1)
         Index Cond: (order_date >= '2025-10-01'::date)
         Index Searches: 1
         Buffers: shared hit=3160 read=947
   ->  Seq Scan on _hyper_1_21_chunk  (cost=0.00..5541.74 rows=191419 width=99) (actual time=0.000..0.000 rows=191419.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3149
   ->  Seq Scan on _hyper_1_40_chunk  (cost=0.00..5563.19 rows=192175 width=99) (actual time=0.000..0.000 rows=192175.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3161
   ->  Seq Scan on _hyper_1_44_chunk  (cost=0.00..5557.98 rows=191998 width=99) (actual time=0.000..0.000 rows=191935.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3158
   ->  Seq Scan on _hyper_1_45_chunk  (cost=0.00..4929.35 rows=170268 width=99) (actual time=0.000..1.003 rows=170222.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=2801
   ->  Seq Scan on _hyper_1_49_chunk  (cost=0.00..5553.95 rows=191836 width=99) (actual time=0.000..0.000 rows=191804.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3156
   ->  Seq Scan on _hyper_1_52_chunk  (cost=0.00..5564.18 rows=192174 width=99) (actual time=0.000..0.000 rows=192159.00 loops=1)
         Filter: (order_date >= '2025-10-01'::date)
         Buffers: shared hit=3162
 Planning:
   Buffers: shared hit=270 read=32
 Planning Time: 6.018 ms
 Execution Time: 203.625 ms
(31 rows)

Time: 210.647 ms

```