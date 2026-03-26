CREATE OR REPLACE FUNCTION ensure_delivery_attempt_partitions(months_ahead INT DEFAULT 2)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    i INT;
    from_date DATE;
    to_date DATE;
    part_name TEXT;
BEGIN
    FOR i IN 0..months_ahead LOOP
        from_date := (date_trunc('month', now()) + make_interval(months => i))::date;
        to_date := (date_trunc('month', now()) + make_interval(months => i + 1))::date;
        part_name := format('delivery_attempts_%s', to_char(from_date, 'YYYYMM'));

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF delivery_attempts FOR VALUES FROM (%L) TO (%L)',
            part_name, from_date, to_date
        );
    END LOOP;
END $$;
