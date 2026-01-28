// Timezone-aware Timestamp Component for Fern Platform
// Displays timestamps in local timezone with UTC hover tooltip

class TimestampComponent {
    constructor() {
        this.userTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
        this.initializeStyles();
    }

    initializeStyles() {
        // Add styles for the component if they don't exist
        // Guard against SSR/Node contexts where document is undefined
        if (typeof window === 'undefined' || !document.getElementById('timestamp-component-styles')) {
            const style = document.createElement('style');
            style.id = 'timestamp-component-styles';
            style.textContent = `
                .timestamp-wrapper {
                    position: relative;
                    display: inline-block;
                    cursor: help;
                }

                .timestamp-local {
                    color: inherit;
                    font-family: inherit;
                }

                .timestamp-icon {
                    margin-left: 4px;
                    opacity: 0.6;
                    font-size: 0.85em;
                    color: var(--text-muted, #6b7280);
                }

                .timestamp-tooltip {
                    position: absolute;
                    top: -35px;
                    left: 50%;
                    transform: translateX(-50%);
                    background: rgba(0, 0, 0, 0.9);
                    color: white;
                    padding: 6px 12px;
                    border-radius: 6px;
                    font-size: 12px;
                    white-space: nowrap;
                    opacity: 0;
                    visibility: hidden;
                    transition: opacity 0.3s, visibility 0.3s;
                    z-index: 1000;
                    pointer-events: none;
                }

                .timestamp-tooltip::after {
                    content: '';
                    position: absolute;
                    top: 100%;
                    left: 50%;
                    transform: translateX(-50%);
                    border: 5px solid transparent;
                    border-top-color: rgba(0, 0, 0, 0.9);
                }

                .timestamp-wrapper:hover .timestamp-tooltip {
                    opacity: 1;
                    visibility: visible;
                    transition-delay: 300ms;
                }
            `;
            document.head.appendChild(style);
        }
    }

    /**
     * Format a timestamp for display in local timezone
     * @param {string|Date} timestamp - ISO timestamp or Date object
     * @param {Object} options - Formatting options
     * @returns {Object} - Object with localFormatted and utcFormatted strings
     */
    formatTimestamp(timestamp, options = {}) {
        const {
            includeSeconds = true,
            shortFormat = false
        } = options;

        let date;
        if (typeof timestamp === 'string') {
            date = new Date(timestamp);
        } else if (timestamp instanceof Date) {
            date = timestamp;
        } else {
            throw new Error('Invalid timestamp format');
        }

        if (isNaN(date.getTime())) {
            throw new Error('Invalid date');
        }

        // Format for local timezone
        const localOptions = {
            year: 'numeric',
            month: shortFormat ? 'short' : 'long',
            day: 'numeric',
            hour: 'numeric',
            minute: '2-digit',
            timeZoneName: 'short'
        };

        if (includeSeconds) {
            localOptions.second = '2-digit';
        }

        const localFormatted = new Intl.DateTimeFormat('en-US', {
            ...localOptions,
            timeZone: this.userTimezone
        }).format(date);

        // Format for UTC tooltip
        const utcFormatted = new Intl.DateTimeFormat('en-US', {
            ...localOptions,
            timeZone: 'UTC',
            timeZoneName: 'short'
        }).format(date);

        return { localFormatted, utcFormatted };
    }

    /**
     * Create a DOM element with timezone-aware timestamp display
     * @param {string|Date} timestamp - ISO timestamp or Date object
     * @param {Object} options - Formatting and display options
     * @returns {HTMLElement} - DOM element with timestamp and hover tooltip
     */
    createElement(timestamp, options = {}) {
        const { localFormatted, utcFormatted } = this.formatTimestamp(timestamp, options);

        const wrapper = document.createElement('span');
        wrapper.className = 'timestamp-wrapper';

        const timestampSpan = document.createElement('span');
        timestampSpan.className = 'timestamp-local';
        timestampSpan.textContent = localFormatted;

        const icon = document.createElement('span');
        icon.className = 'timestamp-icon';
        icon.innerHTML = '🌍';
        icon.setAttribute('aria-hidden', 'true');

        const tooltip = document.createElement('div');
        tooltip.className = 'timestamp-tooltip';
        tooltip.textContent = `UTC: ${utcFormatted}`;
        tooltip.setAttribute('role', 'tooltip');

        wrapper.appendChild(timestampSpan);
        wrapper.appendChild(icon);
        wrapper.appendChild(tooltip);

        // Add ARIA attributes for accessibility
        wrapper.setAttribute('aria-label', `Local time: ${localFormatted}, UTC time: ${utcFormatted}`);
        wrapper.setAttribute('tabindex', '0');

        return wrapper;
    }

    /**
     * Replace all timestamps in a container with timezone-aware components
     * @param {HTMLElement} container - Container element to search for timestamps
     * @param {string} selector - CSS selector for timestamp elements
     * @param {Object} options - Formatting options
     */
    replaceTimestampsInContainer(container, selector = '[data-timestamp]', options = {}) {
        const timestampElements = container.querySelectorAll(selector);
        
        timestampElements.forEach(element => {
            const timestamp = element.dataset.timestamp || element.textContent.trim();
            if (timestamp) {
                try {
                    const newElement = this.createElement(timestamp, options);
                    element.parentNode.replaceChild(newElement, element);
                } catch (error) {
                    console.warn('Failed to parse timestamp:', timestamp, error);
                }
            }
        });
    }

    /**
     * Format a relative time (e.g., "2 hours ago") with timezone awareness
     * @param {string|Date} timestamp - ISO timestamp or Date object
     * @returns {Object} - Object with relative time and tooltip
     */
    formatRelativeTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffMinutes = Math.floor(diffMs / (1000 * 60));
        const diffHours = Math.floor(diffMinutes / 60);
        const diffDays = Math.floor(diffHours / 24);

        let relativeText;
        if (diffMinutes < 1) {
            relativeText = 'Just now';
        } else if (diffMinutes < 60) {
            relativeText = `${diffMinutes} minute${diffMinutes === 1 ? '' : 's'} ago`;
        } else if (diffHours < 24) {
            relativeText = `${diffHours} hour${diffHours === 1 ? '' : 's'} ago`;
        } else if (diffDays < 7) {
            relativeText = `${diffDays} day${diffDays === 1 ? '' : 's'} ago`;
        } else {
            const { localFormatted } = this.formatTimestamp(timestamp, { shortFormat: true });
            relativeText = localFormatted;
        }

        const { localFormatted, utcFormatted } = this.formatTimestamp(timestamp);
        
        return {
            relativeText,
            localFormatted,
            utcFormatted
        };
    }

    /**
     * Create a relative timestamp element
     * @param {string|Date} timestamp - ISO timestamp or Date object
     * @returns {HTMLElement} - DOM element with relative time and hover tooltip
     */
    createRelativeElement(timestamp) {
        const { relativeText, localFormatted, utcFormatted } = this.formatRelativeTime(timestamp);

        const wrapper = document.createElement('span');
        wrapper.className = 'timestamp-wrapper';

        const timestampSpan = document.createElement('span');
        timestampSpan.className = 'timestamp-local';
        timestampSpan.textContent = relativeText;

        const icon = document.createElement('span');
        icon.className = 'timestamp-icon';
        icon.innerHTML = '🕐';
        icon.setAttribute('aria-hidden', 'true');

        const tooltip = document.createElement('div');
        tooltip.className = 'timestamp-tooltip';
        tooltip.innerHTML = `Local: ${localFormatted}<br>UTC: ${utcFormatted}`;
        tooltip.setAttribute('role', 'tooltip');

        wrapper.appendChild(timestampSpan);
        wrapper.appendChild(icon);
        wrapper.appendChild(tooltip);

        // Add ARIA attributes for accessibility
        wrapper.setAttribute('aria-label', `${relativeText}, Local time: ${localFormatted}, UTC time: ${utcFormatted}`);
        wrapper.setAttribute('tabindex', '0');

        return wrapper;
    }

    /**
     * Get user's timezone information
     * @returns {Object} - Timezone information
     */
    getTimezoneInfo() {
        const timeZone = this.userTimezone;
        const now = new Date();
        const formatter = new Intl.DateTimeFormat('en-US', {
            timeZone,
            timeZoneName: 'short'
        });
        
        const parts = formatter.formatToParts(now);
        const timeZoneName = parts.find(part => part.type === 'timeZoneName')?.value || timeZone;

        return {
            timeZone,
            timeZoneName,
            offset: -now.getTimezoneOffset() / 60
        };
    }
}

// Create global instance with SSR compatibility
if (typeof window !== 'undefined') {
    window.TimestampComponent = new TimestampComponent();
}

// Export for module usage if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimestampComponent;
}