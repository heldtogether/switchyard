import "@testing-library/jest-dom";
import { act } from "@testing-library/react";
import { notifyManager } from "@tanstack/query-core";
import { afterAll, beforeAll, vi } from "vitest";

notifyManager.setNotifyFunction((fn) => {
  act(fn);
});

notifyManager.setBatchNotifyFunction((fn) => {
  act(fn);
});

const originalConsoleError = console.error.bind(console);
const originalConsoleWarn = console.warn.bind(console);

beforeAll(() => {
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    const message = String(args[0] ?? "");
    if (message.includes("not wrapped in act")) {
      return;
    }
    originalConsoleError(...args);
  });

  vi.spyOn(console, "warn").mockImplementation((...args: unknown[]) => {
    const message = String(args[0] ?? "");
    if (message.includes("React Router Future Flag Warning")) {
      return;
    }
    originalConsoleWarn(...args);
  });
});

afterAll(() => {
  vi.restoreAllMocks();
});
