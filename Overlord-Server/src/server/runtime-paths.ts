import path from "path";
import fs from "fs";

export function resolveRuntimeRoot(cwd: string = process.cwd()): string {
  if (fs.existsSync("/app/dist-clients") && fs.existsSync("/app/Overlord-Server")) {
    return "/app";
  }

  const explicitRoot = process.env.OVERLORD_ROOT?.trim();
  if (explicitRoot) {
    const resolved = path.resolve(explicitRoot);
    if (path.basename(resolved) === "Overlord-Server" && fs.existsSync(path.join(resolved, "..", "dist-clients"))) {
      return path.resolve(resolved, "..");
    }
    return resolved;
  }

  if (fs.existsSync(path.join(cwd, "Overlord-Client"))) {
    return cwd;
  }

  return path.resolve(cwd, "..");
}
