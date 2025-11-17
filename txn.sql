
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
) WITH (
    tsdb.hypertable,
    tsdb.partition_column = 'order_date',
    tsdb.create_default_indexes = false,
    tsdb.segmentby        = 'sender_account_id',
    tsdb.orderby          = 'order_date DESC'
);

CREATE UNIQUE INDEX idx_payment_order_ref_num ON payment_orders(reference_number, order_date);

insert into payment_orders (sender_account_id, beneficiary_account_id, amount, currency_code, reference_number, payment_method, scheduled_execution_date) values
(1001, 2001, 1500.00, 'USD', 'REF123456', 'Wire Transfer', '2025-12-01'),
(1002, 2002, 2500.50, 'EUR', 'REF123457', 'ACH', '2025-12-02'),
(1003, 2003, 500.75, 'GBP', 'REF123458', 'Check', '2025-12-03'),
(1004, 2004, 3000.00, 'USD', 'REF123459', 'Wire Transfer', '2025-12-04'),
(1005, 2005, 1200.25, 'JPY', 'REF123460', 'ACH', '2025-12-05');

-- always use order_date when updating/deleting records
update payment_orders set status = 2, updated_at = CURRENT_TIMESTAMP where order_date = uuid_extract_timestamp('019a90ee-8063-7de7-8c6a-da2f6518934a')::DATE and id = '019a90ee-8063-7de7-8c6a-da2f6518934a';


-- let check duplicate reference number constraint 
-- it has limitation for UNIQUE constraint only one day
insert into payment_orders (sender_account_id, beneficiary_account_id, amount, currency_code, reference_number, payment_method, scheduled_execution_date) values
(1006, 2006, 1800.00, 'USD', 'REF123456', 'Wire Transfer', '2025-12-06');


-- when select use allway order_date filter for better performance
select * from payment_orders where order_date = '2025-11-01' AND reference_number = 'REF123456';

select * from payment_orders where order_date > '2025-11-02' AND sender_account_id = 1002;

-- better to use order_date filter for range query
select * from payment_orders where order_date BETWEEN '2025-11-01' AND '2025-11-30' AND amount > 1000.00;