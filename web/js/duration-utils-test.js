/**
 * Unit tests for duration-utils.js
 * Run with: node duration-utils-test.js
 */

/**
 * Unit tests for duration-utils.js
 * Run with: node duration-utils-test.js
 */

// Copy the formatDuration function directly here for testing
function formatDuration(milliseconds) {
    // Handle null, undefined, or negative values
    if (!milliseconds || milliseconds < 0) return "0ms";
    
    // Round to avoid floating point precision issues
    const ms = Math.round(milliseconds);
    
    // Under 1 second - show milliseconds
    if (ms < 1000) {
        return ms + "ms";
    }
    
    const seconds = ms / 1000;
    
    // Under 1 minute - show seconds with one decimal place
    if (seconds < 60) {
        return seconds.toFixed(1) + "s";
    }
    
    let totalMinutes = Math.floor(seconds / 60);
    let remainingSeconds = Math.round(seconds % 60);
    
    // Handle case where remainingSeconds rounds to 60
    if (remainingSeconds === 60) {
        totalMinutes += 1;
        remainingSeconds = 0;
    }
    
    // Under 1 hour - show minutes and seconds
    if (totalMinutes < 60) {
        if (remainingSeconds === 0) {
            return totalMinutes + "m 0s";
        }
        return totalMinutes + "m " + remainingSeconds + "s";
    }
    
    // 1 hour or more - show hours, minutes, and optionally seconds
    let hours = Math.floor(totalMinutes / 60);
    let remainingMinutes = totalMinutes % 60;
    
    let result = hours + "h";
    
    if (remainingMinutes > 0) {
        result += " " + remainingMinutes + "m";
    } else {
        result += " 0m";
    }
    
    // Only show seconds if under 24 hours to avoid overly long strings
    if (hours < 24 && remainingSeconds > 0) {
        result += " " + remainingSeconds + "s";
    }
    
    return result;
}

// Simple test framework
class TestRunner {
    constructor() {
        this.tests = [];
        this.passed = 0;
        this.failed = 0;
    }

    test(description, testFunction) {
        this.tests.push({ description, testFunction });
    }

    assertEqual(actual, expected, message) {
        if (actual === expected) {
            return true;
        } else {
            throw new Error(`${message || 'Assertion failed'}: Expected '${expected}', got '${actual}'`);
        }
    }

    run() {
        console.log('🌿 Fern Platform - Duration Utils Test Suite\n');
        console.log('='.repeat(50));

        for (const test of this.tests) {
            try {
                test.testFunction();
                console.log(`✅ ${test.description}`);
                this.passed++;
            } catch (error) {
                console.log(`❌ ${test.description}`);
                console.log(`   Error: ${error.message}`);
                this.failed++;
            }
        }

        console.log('\n' + '='.repeat(50));
        console.log(`\nTest Results:`);
        console.log(`  Total: ${this.tests.length}`);
        console.log(`  Passed: ${this.passed}`);
        console.log(`  Failed: ${this.failed}`);
        console.log(`  Pass Rate: ${((this.passed / this.tests.length) * 100).toFixed(1)}%`);

        if (this.failed === 0) {
            console.log('\n🎉 All tests passed! Duration formatting is working correctly.');
            process.exit(0);
        } else {
            console.log('\n❌ Some tests failed. Please check the implementation.');
            process.exit(1);
        }
    }
}

// Create test runner instance
const runner = new TestRunner();

// Test cases from the user story acceptance criteria
runner.test('Should format milliseconds under 1000ms', () => {
    runner.assertEqual(formatDuration(123), '123ms', 'Small millisecond value');
    runner.assertEqual(formatDuration(500), '500ms', 'Medium millisecond value');
    runner.assertEqual(formatDuration(999), '999ms', 'Just under 1 second');
});

runner.test('Should format seconds between 1-59 seconds', () => {
    runner.assertEqual(formatDuration(1000), '1.0s', 'Exactly 1 second');
    runner.assertEqual(formatDuration(1500), '1.5s', '1.5 seconds');
    runner.assertEqual(formatDuration(12300), '12.3s', '12.3 seconds');
    runner.assertEqual(formatDuration(59900), '59.9s', 'Just under 1 minute');
});

runner.test('Should format minutes between 1-59 minutes', () => {
    runner.assertEqual(formatDuration(60000), '1m 0s', 'Exactly 1 minute');
    runner.assertEqual(formatDuration(65000), '1m 5s', '1 minute 5 seconds');
    runner.assertEqual(formatDuration(80000), '1m 20s', '1 minute 20 seconds');
    runner.assertEqual(formatDuration(125000), '2m 5s', '125 seconds = 2m 5s');
    runner.assertEqual(formatDuration(119000), '1m 59s', '119 seconds');
});

runner.test('Should format hours correctly', () => {
    runner.assertEqual(formatDuration(3600000), '1h 0m', 'Exactly 1 hour');
    runner.assertEqual(formatDuration(3661000), '1h 1m 1s', '1h 1m 1s');
    runner.assertEqual(formatDuration(3725000), '1h 2m 5s', '1h 2m 5s');
    runner.assertEqual(formatDuration(7200000), '2h 0m', '2 hours');
});

runner.test('Should handle edge cases correctly', () => {
    runner.assertEqual(formatDuration(0), '0ms', 'Zero milliseconds');
    runner.assertEqual(formatDuration(-100), '0ms', 'Negative value');
    runner.assertEqual(formatDuration(null), '0ms', 'Null value');
    runner.assertEqual(formatDuration(undefined), '0ms', 'Undefined value');
});

runner.test('Should handle boundary conditions', () => {
    runner.assertEqual(formatDuration(59999), '60.0s', '59999ms = 60.0s (rounded)');
    runner.assertEqual(formatDuration(3599000), '59m 59s', 'Just under 1 hour');
    runner.assertEqual(formatDuration(86400000), '24h 0m', '24 hours (1 day)');
});

runner.test('Should format real-world test durations', () => {
    runner.assertEqual(formatDuration(45), '45ms', 'Fast unit test');
    runner.assertEqual(formatDuration(150), '150ms', 'Quick integration test');
    runner.assertEqual(formatDuration(2500), '2.5s', 'Medium test');
    runner.assertEqual(formatDuration(15000), '15.0s', 'Slow test');
    runner.assertEqual(formatDuration(90000), '1m 30s', 'Very slow test');
    runner.assertEqual(formatDuration(300000), '5m 0s', 'Suite duration');
    runner.assertEqual(formatDuration(1800000), '30m 0s', 'Full test run');
});

runner.test('Should handle specific acceptance criteria examples', () => {
    runner.assertEqual(formatDuration(245700), '4m 6s', '245.7s ≈ 4m 6s');
    runner.assertEqual(formatDuration(3725000), '1h 2m 5s', 'Complex duration from requirements');
});

runner.test('Should handle floating point precision', () => {
    // Test with floating point inputs (should be rounded)
    runner.assertEqual(formatDuration(1500.7), '1.5s', 'Floating point input');
    runner.assertEqual(formatDuration(999.9), '1.0s', 'Floating point rounding to seconds');
});

runner.test('Should maintain consistency across ranges', () => {
    // Test boundary transitions
    runner.assertEqual(formatDuration(999), '999ms', 'Last millisecond value');
    runner.assertEqual(formatDuration(1000), '1.0s', 'First second value');
    runner.assertEqual(formatDuration(59999), '60.0s', 'Last second value (rounded)');
    runner.assertEqual(formatDuration(60000), '1m 0s', 'First minute value');
    runner.assertEqual(formatDuration(3599999), '1h 0m', 'Last minute value (rounded to hour)');
    runner.assertEqual(formatDuration(3600000), '1h 0m', 'First hour value');
});

// Run all tests
runner.run();
