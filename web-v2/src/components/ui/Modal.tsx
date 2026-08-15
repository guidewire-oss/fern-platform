import { useEffect, useRef, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/cn';

interface Props {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string | undefined;
  children: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
}

const SIZES = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-2xl',
} as const;

export function Modal({ open, onClose, title, description, children, footer, size = 'md' }: Props) {
  const pressStart = useRef<EventTarget | null>(null);
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    // Backdrop press-and-release + window-level Escape both dismiss the
    // modal. The backdrop is a non-interactive surface — keyboard users
    // dismiss via Escape (registered above), so the pointer handling here
    // is a mouse-only convenience.
    //
    // It keys off mousedown/mouseup rather than click: a click targets the
    // common ancestor of its press and release, so a drag started inside the
    // dialog and released out here would arrive as a backdrop click and
    // dismiss mid-selection. Both ends must land on the backdrop, and the
    // button check keeps right-clicks out.
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 backdrop-blur-sm"
      onMouseDown={(e) => {
        pressStart.current = e.target;
      }}
      onMouseUp={(e) => {
        if (e.button !== 0) return;
        const startedOnBackdrop = pressStart.current === e.currentTarget;
        pressStart.current = null;
        if (startedOnBackdrop && e.target === e.currentTarget) onClose();
      }}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
    >
      <div
        className={cn('w-full mx-4 rounded-xl border border-border bg-surface shadow-xl', SIZES[size])}
      >
        <header className="flex items-start justify-between border-b border-border px-4 py-3">
          <div>
            <h2 id="modal-title" className="text-base font-semibold">{title}</h2>
            {description && <p className="mt-0.5 text-xs text-muted">{description}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="px-4 py-4">{children}</div>
        {footer && (
          <footer className="flex items-center justify-end gap-2 border-t border-border bg-surface-2/50 px-4 py-2.5">
            {footer}
          </footer>
        )}
      </div>
    </div>
  );
}
