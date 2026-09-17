-- Reset id sequences to MAX(id) for all seeded tables.
--
-- Seed migrations insert rows with explicit ids (menus, apis, articles,
-- categories, dict, configs, ...), which leaves the backing sequences at 1.
-- On any fresh install the first INSERT without an id then violates the
-- primary key (SQLSTATE 23505) and the create endpoint returns 500 —
-- observed live for content categories and articles.
--
-- Note: sequences AHEAD of MAX(id) (manual setval / restored dump) are also
-- rewound to MAX(id); id reuse is impossible because existing rows own those ids.
-- Idempotent: aligns every sequence to COALESCE(MAX(id),1); no-op when
-- already in sync. Covers both fresh installs and existing deployments.

DO $$
DECLARE r RECORD; seqname TEXT; maxid BIGINT;
BEGIN
    FOR r IN
        SELECT table_name
        FROM information_schema.tables t
        WHERE table_schema = 'public'
          AND table_type = 'BASE TABLE'
          AND EXISTS (
              SELECT 1 FROM information_schema.columns c
              WHERE c.table_schema = 'public'
                AND c.table_name = t.table_name
                AND c.column_name = 'id'
          )
    LOOP
        seqname := pg_get_serial_sequence(format('public.%I', r.table_name), 'id');
        CONTINUE WHEN seqname IS NULL;
        EXECUTE format('SELECT COALESCE(MAX(id), 0) FROM public.%I', r.table_name) INTO maxid;
        CONTINUE WHEN maxid = 0;
        PERFORM setval(seqname, GREATEST(maxid, 1));
    END LOOP;
END $$;
