import { type ButtonHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
type Size = 'sm' | 'md';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
};

// Each variant carries its own border + background so the button stays
// visible on every surface — including modal footers (bg-surface-2/50)
// where a pure-white button on a light gray vanishes.
const VARIANTS: Record<Variant, string> = {
  primary:
    'bg-primary text-white border border-primary shadow-sm ' +
    'hover:bg-primary-hover hover:border-primary-hover',
  secondary:
    'bg-slate-100 text-slate-900 border border-slate-300 shadow-sm ' +
    'hover:bg-slate-200 hover:border-slate-400 ' +
    'dark:bg-slate-800 dark:text-slate-100 dark:border-slate-600 dark:hover:bg-slate-700',
  ghost:
    'bg-transparent text-foreground border border-transparent ' +
    'hover:bg-slate-100 dark:hover:bg-slate-800',
  danger:
    'bg-red-600 text-white border border-red-700 shadow-sm ' +
    'hover:bg-red-700 hover:border-red-800',
};

const SIZES: Record<Size, string> = {
  sm: 'px-2 py-1 text-xs',
  md: 'px-3 py-1.5 text-sm',
};

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ variant = 'primary', size = 'md', className, ...rest }, ref) => (
    <button
      ref={ref}
      className={cn(
        'inline-flex items-center justify-center gap-1.5 rounded font-medium',
        'transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
        'focus:outline-none focus:ring-2 focus:ring-primary/40',
        VARIANTS[variant],
        SIZES[size],
        className,
      )}
      {...rest}
    />
  ),
);
Button.displayName = 'Button';
