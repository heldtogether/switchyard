const RESERVED_UI_SLUGS = new Set([
  "runs",
  "jobs",
  "artefacts",
  "billing",
  "executors",
  "settings",
  "login",
  "accept-invite",
  "api"
]);

export function isReservedSlug(slug: string): boolean {
  return RESERVED_UI_SLUGS.has(slug.trim().toLowerCase());
}
