import { afterEach, beforeEach, vi } from "vitest";

const storage = new Map<string, string>();
const localStorageMock: Storage = {
  getItem: (key) => storage.get(key) ?? null,
  setItem: (key, value) => void storage.set(key, String(value)),
  removeItem: (key) => void storage.delete(key),
  clear: () => storage.clear(),
  key: (index) => [...storage.keys()][index] ?? null,
  get length() {
    return storage.size;
  },
};

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// Unit tests run in jsdom. Mock browser-only integrations at their import
// boundary, and reset globals/storage here so individual tests stay isolated.
// Node 26 exposes an unavailable process-wide localStorage by default; jsdom's
// implementation is therefore supplied explicitly for deterministic tests.
beforeEach(() => {
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: localStorageMock,
  });
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: localStorageMock,
  });
  Object.defineProperty(globalThis, "ResizeObserver", {
    configurable: true,
    value: ResizeObserverMock,
  });
  Object.defineProperty(window, "ResizeObserver", {
    configurable: true,
    value: ResizeObserverMock,
  });
  Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", {
    configurable: true,
    value: true,
  });
});

afterEach(() => {
  localStorageMock.clear();
  vi.unstubAllGlobals();
});
