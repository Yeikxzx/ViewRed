import '@testing-library/jest-dom';

// jsdom does not implement scrollIntoView. Provide a no-op shim so views
// that call element.scrollIntoView() during mount (e.g. AsistenteView)
// do not throw under Vitest. Production behavior is unaffected.
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function scrollIntoView() {
    /* no-op for jsdom */
  };
}
