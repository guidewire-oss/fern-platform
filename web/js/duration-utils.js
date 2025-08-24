/**
 * Duration formatting utility for Fern Platform
 * Formats milliseconds into human-readable duration strings
 */

/**
 * Formats duration from milliseconds to human-readable format
 * @param {number} milliseconds - Duration in milliseconds
 * @returns {string} Human-readable duration string
 * 
 * Examples:
 * - formatDuration(500) => "500ms"
 * - formatDuration(1500) => "1.5s"
 * - formatDuration(65000) => "1m 5s"
 * - formatDuration(3725000) => "1h 2m 5s"
 */
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

// Export for ES6 modules if available, otherwise attach to the window 
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { formatDuration };
} else if (typeof window !== 'undefined') {
    window.formatDuration = formatDuration;
    
    // Also create a namespace for better organization
    if (!window.FernUtils) {
        window.FernUtils = {};
    }
    window.FernUtils.formatDuration = formatDuration;
}