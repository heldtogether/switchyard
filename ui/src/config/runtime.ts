export function getRuntimeEnv() {
  return window.__ENV ?? {};
}

export function getAppVersion() {
  const runtimeEnv = getRuntimeEnv();
  const version = runtimeEnv.VERSION ?? import.meta.env.VITE_VERSION ?? "";
  const trimmed = version.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}
