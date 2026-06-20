import '@testing-library/jest-dom';

// jsdom does not implement scrollIntoView. Provide a no-op shim so views
// that call element.scrollIntoView() during mount (e.g. AsistenteView)
// do not throw under Vitest. Production behavior is unaffected.
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function scrollIntoView() {
    /* no-op for jsdom */
  };
}

if (typeof global.ResizeObserver === 'undefined') {
  global.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

if (typeof global.EventSource === 'undefined') {
  global.EventSource = class EventSource {
    url: string;
    readyState: number = 0;
    CONNECTING: number = 0;
    OPEN: number = 1;
    CLOSED: number = 2;
    onopen: ((this: EventSource, ev: Event) => any) | null = null;
    onmessage: ((this: EventSource, ev: MessageEvent) => any) | null = null;
    onerror: ((this: EventSource, ev: Event) => any) | null = null;
    withCredentials: boolean = false;

    constructor(url: string) {
      this.url = url;
    }

    close() {}
    addEventListener() {}
    removeEventListener() {}
    dispatchEvent() { return true; }
  } as any;
}
