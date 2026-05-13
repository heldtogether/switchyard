import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import { execFileSync } from "node:child_process";

function gitOutput(args: string[]) {
  return execFileSync("git", args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"]
  }).trim();
}

function defaultLocalVersion() {
  try {
    const sha = gitOutput(["rev-parse", "--short", "HEAD"]);
    const tag = gitOutput(["describe", "--tags", "--abbrev=0"]);
    const exactTags = gitOutput(["tag", "--points-at", "HEAD"]);
    const status = gitOutput(["status", "--porcelain"]);
    const dirtySuffix = status ? "-dirty" : "";

    if (exactTags.split("\n").includes(tag) && !status) {
      return tag;
    }

    return `${tag}+sha.${sha}${dirtySuffix}`;
  } catch {
    return "";
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  process.env.VITE_VERSION = process.env.VITE_VERSION || env.VITE_VERSION || defaultLocalVersion();

  return {
    plugins: [react()],
    server: {
      port: 5173
    }
  };
});
