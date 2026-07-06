import { Toaster, sileo } from 'sileo';
import type { ReactNode } from 'react';

interface NotifyOptions {
  description?: ReactNode | string;
  duration?: number | null;
}

interface ConfirmOptions extends NotifyOptions {
  actionLabel: string;
  onAction: () => void | Promise<void>;
}

const shared = {
  fill: '#111823',
  roundness: 14,
  styles: {
    title: 'text-[#f8fafc]!',
    description: 'text-[#b9c4d5]!',
    badge: 'bg-white/10!',
    button: 'bg-[#1c2b42]! text-[#60a5fa]! hover:bg-[#263a59]!',
  },
};

function normalizeMessage(title: string, options: NotifyOptions = {}) {
  return {
    title,
    description: options.description,
    duration: options.duration,
    ...shared,
  };
}

export function AppToaster() {
  return (
    <Toaster
      theme="dark"
      position="top-right"
      offset={{ top: 18, right: 18 }}
      options={{
        ...shared,
        duration: 6500,
        autopilot: { expand: 600, collapse: 5200 },
      }}
    />
  );
}

export const notify = {
  success(title: string, options?: NotifyOptions) {
    return sileo.success(normalizeMessage(title, options));
  },
  error(title: string, options?: NotifyOptions) {
    return sileo.error(normalizeMessage(title, options));
  },
  warning(title: string, options?: NotifyOptions) {
    return sileo.warning(normalizeMessage(title, options));
  },
  info(title: string, options?: NotifyOptions) {
    return sileo.info(normalizeMessage(title, options));
  },
  confirm(title: string, options: ConfirmOptions) {
    return sileo.action({
      ...normalizeMessage(title, options),
      duration: options.duration ?? 10000,
      button: {
        title: options.actionLabel,
        onClick: options.onAction,
      },
    });
  },
};
