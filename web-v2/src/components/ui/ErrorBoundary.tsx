import { Component, type ErrorInfo, type ReactNode } from 'react';
import { EmptyState } from './EmptyState';

// Catches render-phase exceptions inside any subtree so a single
// crashing component doesn't blank the whole page. Without this, a
// thrown TypeError in (say) a settings tab unmounts the entire route
// and the user sees an empty `<div>` with no clue what went wrong.
//
// Class component because React's error-boundary contract requires
// componentDidCatch + getDerivedStateFromError — there's no hook
// equivalent yet.

interface Props {
  children: ReactNode;
  // Label used in the fallback message ("Couldn't render <label>").
  // Helps the user understand which subtree failed when multiple
  // boundaries are visible at once.
  label?: string;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  override state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    // Log to the browser console so engineers debugging from DevTools
    // get a usable stack trace. Wired to console.error rather than a
    // telemetry hook because v2 doesn't have one yet.
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  reset = () => {
    this.setState({ error: null });
  };

  override render() {
    if (this.state.error) {
      const label = this.props.label ?? 'this section';
      return (
        <EmptyState
          title={`Couldn't render ${label}`}
          description={this.state.error.message || 'An unexpected error occurred.'}
          action={
            <button
              type="button"
              onClick={this.reset}
              className="rounded border border-border bg-surface px-3 py-1 text-xs hover:bg-surface-2"
            >
              Retry
            </button>
          }
        />
      );
    }
    return this.props.children;
  }
}
