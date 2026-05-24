import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

// forwardRef so consumers can observe the card's box (e.g. with an
// IntersectionObserver) without having to wrap it in another div.
export const Card = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Card({ className, ...rest }, ref) {
    return (
      <div ref={ref} className={cn('fern-card overflow-hidden', className)} {...rest} />
    );
  },
);

export function CardHeader({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'border-b border-border bg-gradient-to-r from-primary-soft to-transparent px-4 py-3',
        className,
      )}
      {...rest}
    />
  );
}

export function CardBody({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('px-4 py-3', className)} {...rest} />;
}

export function CardTitle({ className, children, ...rest }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 className={cn('text-sm font-semibold text-foreground', className)} {...rest}>
      {children}
    </h3>
  );
}
