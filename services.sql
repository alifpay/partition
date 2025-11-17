
-- lets create a table for payments services
CREATE TABLE payments(
    created_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at                  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    pay_date                    DATE DEFAULT CURRENT_DATE,
    id                          UUID  DEFAULT uuidv7(),
    amount                      NUMERIC(18, 2) NOT NULL,
    status                      SMALLINT DEFAULT 1, -- e.g., 1 - created, 2 - processing, 3 - completed, 4 - failed
    error_code                  SMALLINT,
    currency_code               CHAR(3) NOT NULL,
    service_name                TEXT NOT NULL,
    reference_number            TEXT NOT NULL,
    info                        JSONB,
    PRIMARY KEY (id, pay_date)
) WITH (
    tsdb.hypertable,
    tsdb.partition_column = 'pay_date',
    tsdb.create_default_indexes = false,
    tsdb.segmentby        = 'service_name',
    tsdb.orderby          = 'pay_date DESC'
);

-- create unique index on service_name, reference_number and pay_date
-- to ensure uniqueness of reference numbers per service per day so in doc API we can enforce this constraint and mention it in API docs 
CREATE UNIQUE INDEX idx_payment_service_ref_num ON payments(service_name, reference_number, pay_date);

-- optional: make extra worker that before processing to check duplicate reference numbers for greater period if needed
SELECT EXISTS(select 1 from payments where pay_date > '2025-10-01' AND service_name = 'payroll' AND reference_number = 'PAY123456');

