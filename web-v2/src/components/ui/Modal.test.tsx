// Dismissal behavior for Modal (#222). A drag that starts inside the dialog
// and releases over the backdrop must leave it open; a genuine backdrop click
// must still close it. Modal.tsx explains why click semantics make this subtle.

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';

import { Modal } from './Modal';

afterEach(() => {
  cleanup();
});

function renderModal() {
  const onClose = vi.fn();
  render(
    <Modal open onClose={onClose} title="New Project">
      <input aria-label="Name" defaultValue="my-project" />
    </Modal>,
  );
  const backdrop = screen.getByRole('dialog');
  const panel = backdrop.firstElementChild as HTMLElement;
  return { onClose, backdrop, panel };
}

describe('Modal dismissal', () => {
  it('stays open when a drag starts inside the dialog and releases on the backdrop', () => {
    const { onClose, backdrop, panel } = renderModal();

    fireEvent.mouseDown(panel);
    fireEvent.mouseUp(backdrop);
    fireEvent.click(backdrop);

    expect(onClose).not.toHaveBeenCalled();
  });

  it('stays open when a press on the backdrop releases inside the dialog', () => {
    const { onClose, backdrop, panel } = renderModal();

    fireEvent.mouseDown(backdrop);
    fireEvent.mouseUp(panel);
    fireEvent.click(backdrop);

    expect(onClose).not.toHaveBeenCalled();
  });

  it('stays open when press and release are both inside the dialog', () => {
    const { onClose, panel } = renderModal();

    fireEvent.mouseDown(panel);
    fireEvent.mouseUp(panel);
    fireEvent.click(panel);

    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes on a plain backdrop click', () => {
    const { onClose, backdrop } = renderModal();

    fireEvent.mouseDown(backdrop);
    fireEvent.mouseUp(backdrop);
    fireEvent.click(backdrop);

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('ignores non-primary buttons on the backdrop', () => {
    const { onClose, backdrop } = renderModal();

    // Right-click produces mousedown/mouseup but no click, so the old
    // click-based dismissal never fired for it either.
    fireEvent.mouseDown(backdrop, { button: 2 });
    fireEvent.mouseUp(backdrop, { button: 2 });

    expect(onClose).not.toHaveBeenCalled();
  });

  it('does not close on a primary release with no primary press behind it', () => {
    const { onClose, backdrop } = renderModal();

    // A right-click leaves no primary press pending, so a stray primary
    // release — one whose press landed outside the window — must not count.
    fireEvent.mouseDown(backdrop, { button: 2 });
    fireEvent.mouseUp(backdrop, { button: 2 });
    fireEvent.mouseUp(backdrop, { button: 0 });

    expect(onClose).not.toHaveBeenCalled();
  });

  it('closes on Escape', () => {
    const { onClose } = renderModal();

    fireEvent.keyDown(window, { key: 'Escape' });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes via the header close button', () => {
    const { onClose } = renderModal();

    fireEvent.click(screen.getByLabelText('Close'));

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
