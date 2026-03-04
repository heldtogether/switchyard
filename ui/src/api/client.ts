export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

const runtimeEnv = (window as any).__ENV ?? {};
const API_BASE_URL = runtimeEnv.API_BASE_URL ?? import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

export async function fetchJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const data = await res.json();
      message = data?.message ?? data?.error ?? message;
    } catch {
      // ignore
    }
    throw new ApiError(message, res.status);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

export async function fetchText(path: string, init?: RequestInit): Promise<string> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      ...(init?.headers ?? {})
    }
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const data = await res.json();
      message = data?.message ?? data?.error ?? message;
    } catch {
      // ignore
    }
    throw new ApiError(message, res.status);
  }

  return res.text();
}

export function shouldUseMocks(error: unknown) {
  return (
    runtimeEnv.USE_MOCKS === "true" ||
    import.meta.env.VITE_USE_MOCKS === "true" ||
    (error instanceof ApiError && (error.status === 404 || error.status === 501))
  );
}
