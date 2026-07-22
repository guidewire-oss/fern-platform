import { type InputHTMLAttributes, type TextareaHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

// Shared text-input style. The default `bg-white` we had collapses to
// white-on-white in dark mode; using `bg-surface` + `text-foreground`
// keeps the value readable in either scheme.
const BASE = [
  'w-full rounded border border-border',
  'bg-surface text-foreground placeholder:text-muted',
  'px-2.5 py-1.5 text-sm',
  'focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/30',
  'disabled:cursor-not-allowed disabled:opacity-60',
].join(' ');

type InputProps = InputHTMLAttributes<HTMLInputElement>;

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = 'text', ...rest }, ref) => (
    <input ref={ref} type={type} className={cn(BASE, className)} {...rest} />
  ),
);
Input.displayName = 'Input';

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, rows = 3, ...rest }, ref) => (
    <textarea ref={ref} rows={rows} className={cn(BASE, className)} {...rest} />
  ),
);
Textarea.displayName = 'Textarea';

import { type SelectHTMLAttributes } from 'react';
type SelectProps = SelectHTMLAttributes<HTMLSelectElement>;

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, children, ...rest }, ref) => (
    <select ref={ref} className={cn(BASE, className)} {...rest}>
      {children}
    </select>
  ),
);
Select.displayName = 'Select';
