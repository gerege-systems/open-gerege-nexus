import {readFile} from "node:fs/promises";
import {createServer} from "node:http";
import {extname, relative, resolve} from "node:path";

const ROOT = resolve("dist");
const PORT = Number.parseInt(process.env.PORT ?? "4321", 10);
const TYPES = new Map([
  [".css", "text/css; charset=utf-8"],
  [".html", "text/html; charset=utf-8"],
  [".jpg", "image/jpeg"],
  [".jpeg", "image/jpeg"],
  [".png", "image/png"],
  [".svg", "image/svg+xml"],
  [".webp", "image/webp"],
]);

function localPath(requestUrl) {
  const pathname = decodeURIComponent(new URL(requestUrl, "http://localhost").pathname);
  const requested = pathname === "/" ? "index.html" : pathname.replace(/^\/+/, "");
  const target = resolve(ROOT, requested);
  return relative(ROOT, target).startsWith("..") ? null : target;
}

createServer(async (request, response) => {
  try {
    const target = localPath(request.url ?? "/");
    if (!target) {
      response.writeHead(400).end("bad request");
      return;
    }

    const body = await readFile(target);
    response.writeHead(200, {
      "Content-Type": TYPES.get(extname(target)) ?? "application/octet-stream",
      "X-Content-Type-Options": "nosniff",
    });
    response.end(body);
  } catch (error) {
    const status = error?.code === "ENOENT" ? 404 : 500;
    response.writeHead(status, {"Content-Type": "text/plain; charset=utf-8"});
    response.end(status === 404 ? "not found" : "server error");
  }
}).listen(PORT, "127.0.0.1", () => {
  console.log(`http://127.0.0.1:${PORT}`);
});
