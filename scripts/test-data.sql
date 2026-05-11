-- Test Data Generation Script for Fern Platform
-- ==============================================
--
-- Purpose:
-- This SQL script generates comprehensive test data for the Fern Platform database.
-- It creates a realistic dataset that simulates test execution patterns across
-- different types of software projects, providing data for all dashboard views,
-- analytics features, trend analysis, and performance testing.
--
-- The script generates ~100-120 test runs with nested suite and spec data to ensure
-- GraphQL queries are tested at production-like scale.
--
-- Data Generated:
-- 1. Projects (3 total):
--    - E-Commerce Frontend: React/TypeScript project with good test coverage
--    - API Gateway Service: Go project with performance issues (lower pass rate)
--    - Mobile Banking App: React Native project with stable test results
--
-- 2. Test Runs:
--    - 30-40 runs per project spanning several days
--    - Mix of completed, failed, and running statuses
--    - Realistic execution times and branch names
--    - Total: ~100-120 test runs across all projects
--
-- 3. Test Suites:
--    - 2-5 suites per test run
--    - Project-specific suite names (Component/Integration/E2E for frontend,
--      API/Performance/Security for backend, UI/Device/Network for mobile)
--    - Aggregated pass/fail/skip counts
--
-- 4. Test Specs:
--    - 5-10 specs per suite
--    - Realistic test names based on project type
--    - Error messages and stack traces for failed tests
--    - Varying execution times (100ms - 5s)
--
-- Key Features:
-- - Idempotent: Uses ON CONFLICT for projects to avoid duplicates
-- - Time-distributed: Creates historical data for trend analysis
-- - Realistic patterns: API Gateway has ~50% pass rate, others have ~80-90%
-- - Dashboard-ready: Provides data for all UI components including treemaps
--
-- Note: Running this script multiple times will create additional test runs,
-- allowing you to build up a larger dataset over time.

-- Clear existing test data (optional - comment out if you want to keep existing data)
-- DELETE FROM spec_runs WHERE suite_run_id IN (SELECT id FROM suite_runs WHERE test_run_id IN (SELECT id FROM test_runs WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')));
-- DELETE FROM suite_runs WHERE test_run_id IN (SELECT id FROM test_runs WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003'));
-- DELETE FROM test_runs WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003');
-- DELETE FROM project_details WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003');

-- Insert sample projects with UUIDs (skip if they already exist)
INSERT INTO project_details (project_id, name, description, repository, default_branch, team, is_active, created_at, updated_at) VALUES
('550e8400-e29b-41d4-a716-446655440001', 'E-Commerce Frontend', 'React-based e-commerce platform frontend with TypeScript', 'https://github.com/example/ecommerce-frontend', 'main', 'fern', true, NOW() - INTERVAL '30 days', NOW()),
('550e8400-e29b-41d4-a716-446655440002', 'API Gateway Service', 'High-performance API gateway built with Go', 'https://github.com/example/api-gateway', 'main', 'fern', true, NOW() - INTERVAL '45 days', NOW()),
('550e8400-e29b-41d4-a716-446655440003', 'Mobile Banking App', 'Cross-platform mobile banking application using React Native', 'https://github.com/example/mobile-banking', 'develop', 'fern', true, NOW() - INTERVAL '60 days', NOW())
ON CONFLICT (project_id) DO UPDATE 
SET team = 'fern', updated_at = NOW();

-- Function to generate test data
DO $$
DECLARE
    project_rec RECORD;
    test_run_id INTEGER;
    suite_run_id INTEGER;
    suite_counter INTEGER;
    spec_counter INTEGER;
    total_tests INTEGER;
    passed_tests INTEGER;
    failed_tests INTEGER;
    skipped_tests INTEGER;
    total_specs INTEGER;
    passed_specs INTEGER;
    failed_specs INTEGER;
    skipped_specs INTEGER;
    test_status VARCHAR;
    run_duration INTEGER;
    spec_duration INTEGER;
BEGIN
    -- Loop through each project
    FOR project_rec IN 
        SELECT project_id, name 
        FROM project_details 
        WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
    LOOP
        -- Create 30-40 test runs per project (for realistic performance testing)
        -- This generates enough data to test query performance at scale
        FOR i IN 1..30 + floor(random() * 11)::int LOOP
            total_tests := 0;
            passed_tests := 0;
            failed_tests := 0;
            skipped_tests := 0;
            total_specs := 0;
            passed_specs := 0;
            failed_specs := 0;
            skipped_specs := 0;
            run_duration := 0;
            
            -- Insert test run
            INSERT INTO test_runs (
                project_id, run_id, status, branch, commit_sha, 
                start_time, end_time, duration_ms, total_tests, 
                passed_tests, failed_tests, skipped_tests, environment,
                created_at, updated_at
            ) VALUES (
                project_rec.project_id,
                gen_random_uuid()::text,
                'completed',
                CASE 
                    WHEN random() < 0.7 THEN 'main'
                    WHEN random() < 0.9 THEN 'develop'
                    ELSE 'feature/test-' || floor(random() * 100)::text
                END,
                substring(md5(random()::text) from 1 for 8),
                NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int),
                NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int) + INTERVAL '1 minute' * (5 + floor(random() * 20)::int),
                0, -- Will update later
                0, -- Will update later
                0, -- Will update later
                0, -- Will update later
                0, -- Will update later
                CASE 
                    WHEN random() < 0.5 THEN 'production'
                    WHEN random() < 0.8 THEN 'staging'
                    ELSE 'development'
                END,
                NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int),
                NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int)
            ) RETURNING id INTO test_run_id;
            
            -- Create 2-5 test suites per test run
            suite_counter := 2 + floor(random() * 4)::int;
            FOR j IN 1..suite_counter LOOP
                -- Reset suite-level counters
                total_specs := 0;
                passed_specs := 0;
                failed_specs := 0;
                skipped_specs := 0;
                
                -- Insert suite run
                INSERT INTO suite_runs (
                    test_run_id, suite_name, status, start_time, end_time,
                    duration_ms, total_specs, passed_specs, failed_specs,
                    skipped_specs, created_at, updated_at
                ) VALUES (
                    test_run_id,
                    CASE project_rec.project_id
                        WHEN '550e8400-e29b-41d4-a716-446655440001' THEN 
                            CASE j
                                WHEN 1 THEN 'Component Tests'
                                WHEN 2 THEN 'Integration Tests'
                                WHEN 3 THEN 'E2E Tests'
                                WHEN 4 THEN 'Unit Tests'
                                ELSE 'Smoke Tests'
                            END
                        WHEN '550e8400-e29b-41d4-a716-446655440002' THEN
                            CASE j
                                WHEN 1 THEN 'API Tests'
                                WHEN 2 THEN 'Performance Tests'
                                WHEN 3 THEN 'Security Tests'
                                WHEN 4 THEN 'Database Tests'
                                ELSE 'Load Tests'
                            END
                        ELSE -- 550e8400-e29b-41d4-a716-446655440003
                            CASE j
                                WHEN 1 THEN 'UI Tests'
                                WHEN 2 THEN 'Device Tests'
                                WHEN 3 THEN 'Network Tests'
                                WHEN 4 THEN 'Offline Tests'
                                ELSE 'Platform Tests'
                            END
                    END,
                    'completed',
                    NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int),
                    NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int) + INTERVAL '1 second' * (30 + floor(random() * 300)::int),
                    0, -- Will update later
                    0, -- Will update later
                    0, -- Will update later
                    0, -- Will update later
                    0, -- Will update later
                    NOW(),
                    NOW()
                ) RETURNING id INTO suite_run_id;
                
                -- Create 5-10 test specs per suite
                spec_counter := 5 + floor(random() * 6)::int;
                FOR k IN 1..spec_counter LOOP
                    -- Determine test status based on project
                    -- API Gateway Service should have ~50% pass rate
                    IF project_rec.project_id = '550e8400-e29b-41d4-a716-446655440002' THEN
                        IF random() < 0.5 THEN
                            test_status := 'passed';
                            passed_tests := passed_tests + 1;
                            passed_specs := passed_specs + 1;
                        ELSIF random() < 0.8 THEN
                            test_status := 'failed';
                            failed_tests := failed_tests + 1;
                            failed_specs := failed_specs + 1;
                        ELSE
                            test_status := 'skipped';
                            skipped_tests := skipped_tests + 1;
                            skipped_specs := skipped_specs + 1;
                        END IF;
                    ELSE
                        -- Other projects have better pass rates
                        IF random() < 0.8 THEN
                            test_status := 'passed';
                            passed_tests := passed_tests + 1;
                            passed_specs := passed_specs + 1;
                        ELSIF random() < 0.95 THEN
                            test_status := 'failed';
                            failed_tests := failed_tests + 1;
                            failed_specs := failed_specs + 1;
                        ELSE
                            test_status := 'skipped';
                            skipped_tests := skipped_tests + 1;
                            skipped_specs := skipped_specs + 1;
                        END IF;
                    END IF;
                    
                    total_tests := total_tests + 1;
                    total_specs := total_specs + 1;
                    spec_duration := 100 + floor(random() * 5000)::int;
                    run_duration := run_duration + spec_duration;
                    
                    -- Insert spec run
                    INSERT INTO spec_runs (
                        suite_run_id, spec_name,
                        status, start_time, end_time, duration_ms, error_message, stack_trace,
                        created_at, updated_at
                    ) VALUES (
                        suite_run_id,
                        CASE project_rec.project_id
                            WHEN '550e8400-e29b-41d4-a716-446655440001' THEN 
                                'should ' || 
                                CASE floor(random() * 10)::int
                                    WHEN 0 THEN 'render component correctly'
                                    WHEN 1 THEN 'handle user interactions'
                                    WHEN 2 THEN 'validate form inputs'
                                    WHEN 3 THEN 'display error messages'
                                    WHEN 4 THEN 'update state on action'
                                    WHEN 5 THEN 'fetch data from API'
                                    WHEN 6 THEN 'handle loading states'
                                    WHEN 7 THEN 'navigate between pages'
                                    WHEN 8 THEN 'apply correct styling'
                                    ELSE 'handle edge cases'
                                END
                            WHEN '550e8400-e29b-41d4-a716-446655440002' THEN
                                'should ' ||
                                CASE floor(random() * 10)::int
                                    WHEN 0 THEN 'route requests correctly'
                                    WHEN 1 THEN 'authenticate users'
                                    WHEN 2 THEN 'validate request payload'
                                    WHEN 3 THEN 'handle rate limiting'
                                    WHEN 4 THEN 'return correct status codes'
                                    WHEN 5 THEN 'transform response data'
                                    WHEN 6 THEN 'handle timeouts gracefully'
                                    WHEN 7 THEN 'cache responses appropriately'
                                    WHEN 8 THEN 'log requests properly'
                                    ELSE 'handle concurrent requests'
                                END
                            ELSE -- 550e8400-e29b-41d4-a716-446655440003
                                'should ' ||
                                CASE floor(random() * 10)::int
                                    WHEN 0 THEN 'render on all screen sizes'
                                    WHEN 1 THEN 'handle touch gestures'
                                    WHEN 2 THEN 'work offline'
                                    WHEN 3 THEN 'sync data when online'
                                    WHEN 4 THEN 'handle biometric auth'
                                    WHEN 5 THEN 'receive push notifications'
                                    WHEN 6 THEN 'access device camera'
                                    WHEN 7 THEN 'handle deep links'
                                    WHEN 8 THEN 'persist user preferences'
                                    ELSE 'handle app lifecycle'
                                END
                        END || ' - test #' || k,
                        test_status,
                        NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int),
                        NOW() - INTERVAL '1 day' * (i + floor(random() * 10)::int) + INTERVAL '1 millisecond' * spec_duration,
                        spec_duration,
                        CASE 
                            WHEN test_status = 'failed' THEN 'Expected value to be true but got false'
                            ELSE NULL
                        END,
                        CASE 
                            WHEN test_status = 'failed' THEN 'Error: Assertion failed\n  at Object.<anonymous> (test.spec.ts:' || (10 + floor(random() * 200)::int)::text || ':15)\n  at runTest (runner.js:123:5)'
                            ELSE NULL
                        END,
                        NOW(),
                        NOW()
                    );
                END LOOP;
                
                -- Update suite run with aggregated data
                EXECUTE 'UPDATE suite_runs 
                SET 
                    duration_ms = $1,
                    total_specs = $2,
                    passed_specs = $3,
                    failed_specs = $4,
                    skipped_specs = $5
                WHERE id = $6'
                USING spec_duration * spec_counter, 
                      total_specs,
                      passed_specs,
                      failed_specs,
                      skipped_specs,
                      suite_run_id;
            END LOOP;
            
            -- Update test run with aggregated data
            EXECUTE 'UPDATE test_runs 
            SET 
                duration_ms = $1,
                total_tests = $2,
                passed_tests = $3,
                failed_tests = $4,
                skipped_tests = $5,
                status = CASE 
                    WHEN $4 > 0 THEN ''failed''
                    WHEN $5 = $2 THEN ''skipped''
                    ELSE ''passed''
                END
            WHERE id = $6'
            USING run_duration, total_tests, passed_tests, failed_tests, skipped_tests, test_run_id;
        END LOOP;
    END LOOP;
END $$;

-- Add some recent test runs for better dashboard display
DO $$
DECLARE
    project_rec RECORD;
    test_run_id INTEGER;
    suite_run_id INTEGER;
    j INTEGER;
BEGIN
    -- Add very recent test runs (within last 24 hours)
    FOR project_rec IN 
        SELECT project_id, name 
        FROM project_details 
        WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
    LOOP
        -- Insert a recent test run
        INSERT INTO test_runs (
            project_id, run_id, status, branch, commit_sha, 
            start_time, end_time, duration_ms, total_tests, 
            passed_tests, failed_tests, skipped_tests, environment,
            created_at, updated_at
        ) VALUES (
            project_rec.project_id,
            gen_random_uuid()::text,
            'running',
            'main',
            substring(md5(random()::text) from 1 for 8),
            NOW() - INTERVAL '30 minutes',
            NULL,
            0,
            0,
            0,
            0,
            0,
            'production',
            NOW() - INTERVAL '30 minutes',
            NOW()
        ) RETURNING id INTO test_run_id;
        
        -- Add 3-5 suites for recent test runs
        FOR j IN 1..(3 + floor(random() * 3)::int) LOOP
            INSERT INTO suite_runs (
                test_run_id, suite_name, status, start_time, end_time,
                duration_ms, total_specs, passed_specs, failed_specs,
                skipped_specs, created_at, updated_at
            ) VALUES (
                test_run_id,
                CASE project_rec.project_id
                    WHEN '550e8400-e29b-41d4-a716-446655440001' THEN 
                        CASE j
                            WHEN 1 THEN 'Component Tests'
                            WHEN 2 THEN 'Integration Tests'
                            WHEN 3 THEN 'E2E Tests'
                            WHEN 4 THEN 'Unit Tests'
                            ELSE 'Smoke Tests'
                        END
                    WHEN '550e8400-e29b-41d4-a716-446655440002' THEN
                        CASE j
                            WHEN 1 THEN 'API Tests'
                            WHEN 2 THEN 'Performance Tests'
                            WHEN 3 THEN 'Security Tests'
                            WHEN 4 THEN 'Database Tests'
                            ELSE 'Load Tests'
                        END
                    ELSE -- 550e8400-e29b-41d4-a716-446655440003
                        CASE j
                            WHEN 1 THEN 'UI Tests'
                            WHEN 2 THEN 'Device Tests'
                            WHEN 3 THEN 'Network Tests'
                            WHEN 4 THEN 'Offline Tests'
                            ELSE 'Platform Tests'
                        END
                END,
                CASE 
                    WHEN j = 1 THEN 'running'
                    WHEN j = 2 AND random() < 0.5 THEN 'running'
                    ELSE 'completed'
                END,
                NOW() - INTERVAL '30 minutes' - INTERVAL '5 minutes' * (j - 1),
                CASE 
                    WHEN j = 1 THEN NULL
                    WHEN j = 2 AND random() < 0.5 THEN NULL
                    ELSE NOW() - INTERVAL '25 minutes' - INTERVAL '5 minutes' * (j - 1)
                END,
                CASE 
                    WHEN j = 1 THEN 0
                    ELSE (300000 + floor(random() * 600000)::int)
                END,
                CASE 
                    WHEN j = 1 THEN 0
                    ELSE (10 + floor(random() * 20)::int)
                END,
                CASE 
                    WHEN j = 1 THEN 0
                    ELSE (8 + floor(random() * 15)::int)
                END,
                CASE 
                    WHEN j = 1 THEN 0
                    ELSE floor(random() * 5)::int
                END,
                CASE 
                    WHEN j = 1 THEN 0
                    ELSE floor(random() * 3)::int
                END,
                NOW(),
                NOW()
            );
        END LOOP;
    END LOOP;
END $$;

-- Update recent running test runs with aggregated counts
UPDATE test_runs tr
SET 
    total_tests = COALESCE((
        SELECT SUM(sr.total_specs)
        FROM suite_runs sr
        WHERE sr.test_run_id = tr.id
    ), 0),
    passed_tests = COALESCE((
        SELECT SUM(sr.passed_specs)
        FROM suite_runs sr
        WHERE sr.test_run_id = tr.id
    ), 0),
    failed_tests = COALESCE((
        SELECT SUM(sr.failed_specs)
        FROM suite_runs sr
        WHERE sr.test_run_id = tr.id
    ), 0),
    skipped_tests = COALESCE((
        SELECT SUM(sr.skipped_specs)
        FROM suite_runs sr
        WHERE sr.test_run_id = tr.id
    ), 0),
    duration_ms = COALESCE((
        SELECT SUM(sr.duration_ms)
        FROM suite_runs sr
        WHERE sr.test_run_id = tr.id
    ), 0)
WHERE tr.status = 'running' 
AND tr.project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003');

-- Display summary
SELECT 
    'Projects created' as metric,
    COUNT(DISTINCT project_id) as count
FROM project_details
WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
UNION ALL
SELECT 
    'Test runs created',
    COUNT(*)
FROM test_runs
WHERE project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
UNION ALL
SELECT 
    'Suite runs created',
    COUNT(*)
FROM suite_runs sr
JOIN test_runs tr ON sr.test_run_id = tr.id
WHERE tr.project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
UNION ALL
SELECT 
    'Spec runs created',
    COUNT(*)
FROM spec_runs spr
JOIN suite_runs sr ON spr.suite_run_id = sr.id
JOIN test_runs tr ON sr.test_run_id = tr.id
WHERE tr.project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003');

-- Show pass rates by project
SELECT 
    pd.name as project_name,
    COUNT(DISTINCT tr.id) as total_test_runs,
    SUM(tr.total_tests) as total_tests,
    SUM(tr.passed_tests) as passed_tests,
    SUM(tr.failed_tests) as failed_tests,
    SUM(tr.skipped_tests) as skipped_tests,
    ROUND(CASE 
        WHEN SUM(tr.total_tests) > 0 
        THEN (SUM(tr.passed_tests)::numeric / SUM(tr.total_tests)::numeric) * 100
        ELSE 0 
    END, 2) as pass_rate_percentage
FROM project_details pd
JOIN test_runs tr ON pd.project_id = tr.project_id
WHERE pd.project_id IN ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440003')
GROUP BY pd.name
ORDER BY pd.name;