import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import type { Answers } from "@ffconvert/shared/api";
import { type WebSocket, WebSocketServer } from "ws";
import { type JobTemplate, type LibraryTree, MockError, MockState } from "./state";

/**
 * Mock ffconvert-web API: REST + WS snapshot feed on one port (default 8080),
 * launched by Playwright's webServer. The Vite dev server proxies /api here.
 *
 * Test-only control plane lives under /api/__test/* so it rides the same proxy.
 */

const PORT = Number(process.env.PORT ?? 8080);
const state = new MockState();
const clients = new Set<WebSocket>();
const wss = new WebSocketServer({ noServer: true });

function broadcast(): void {
  const frame = JSON.stringify(state.list());
  for (const ws of clients) {
    if (ws.readyState === ws.OPEN) ws.send(frame);
  }
}
state.onChange = broadcast;

function writeJSON(res: ServerResponse, status: number, body: unknown): void {
  const data = JSON.stringify(body);
  res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store" });
  res.end(data);
}

async function readBody(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(chunk as Buffer);
  if (chunks.length === 0) return undefined;
  try {
    return JSON.parse(Buffer.concat(chunks).toString());
  } catch {
    throw new MockError(400, "invalid JSON body");
  }
}

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const url = new URL(req.url ?? "/", `http://localhost:${PORT}`);
  const method = req.method ?? "GET";
  const { pathname } = url;

  // -- test-only control plane ----------------------------------------------

  if (pathname === "/api/__test/reset" && method === "POST") {
    state.reset();
    return writeJSON(res, 200, { ok: true });
  }
  if (pathname === "/api/__test/seed-library" && method === "POST") {
    const body = (await readBody(req)) as { tree?: LibraryTree };
    state.setLibrary(body.tree ?? {});
    return writeJSON(res, 200, { ok: true });
  }
  if (pathname === "/api/__test/seed-job" && method === "POST") {
    const body = (await readBody(req)) as { template: JobTemplate; state?: "questions" };
    return writeJSON(res, 200, state.create(body.template, body.state));
  }
  if (pathname === "/api/__test/requests" && method === "GET") {
    return writeJSON(res, 200, state.requests);
  }

  // -- real API surface (body is read exactly once, then routed) -------------

  const body = method === "GET" ? undefined : await readBody(req);
  if (pathname.startsWith("/api/")) {
    state.requests.push({ method, path: pathname, body });
  }

  if (pathname === "/api/library" && method === "GET") {
    return writeJSON(res, 200, state.listLibrary(url.searchParams.get("path") ?? ""));
  }
  if (pathname === "/api/jobs" && method === "POST") {
    const { path } = (body ?? {}) as { path?: string };
    if (!path) throw new MockError(400, "missing path");
    return writeJSON(res, 202, state.create({ path }));
  }
  if (pathname === "/api/jobs" && method === "GET") {
    return writeJSON(res, 200, state.list());
  }
  const answers = pathname.match(/^\/api\/jobs\/([^/]+)\/answers$/);
  if (answers && method === "POST") {
    const { answers: submitted, category } = (body ?? {}) as {
      answers?: Answers[];
      category?: string;
    };
    return writeJSON(
      res,
      200,
      await state.submitAnswers(answers[1], category ?? "", submitted ?? []),
    );
  }
  const cancel = pathname.match(/^\/api\/jobs\/([^/]+)\/cancel$/);
  if (cancel && method === "POST") {
    return writeJSON(res, 200, state.cancel(cancel[1]));
  }
  const remove = pathname.match(/^\/api\/jobs\/([^/]+)$/);
  if (remove && method === "DELETE") {
    state.remove(remove[1]);
    return writeJSON(res, 200, { removed: true });
  }

  throw new MockError(404, `no route: ${method} ${pathname}`);
}

const server = createServer((req, res) => {
  route(req, res).catch((err) => {
    if (err instanceof MockError) {
      writeJSON(res, err.status, { error: err.message });
    } else {
      console.error(err);
      writeJSON(res, 500, { error: "mock server error" });
    }
  });
});

server.on("upgrade", (req, socket, head) => {
  const { pathname } = new URL(req.url ?? "/", `http://localhost:${PORT}`);
  if (pathname !== "/api/ws") {
    socket.destroy();
    return;
  }
  wss.handleUpgrade(req, socket, head, (ws) => {
    clients.add(ws);
    ws.on("close", () => clients.delete(ws));
    // Client→server frames are ignored, like the real server's CloseRead.
    ws.on("message", () => {});
    ws.send(JSON.stringify(state.list()));
  });
});

// Periodic push regardless of changes, mirroring the real 500ms ticker.
setInterval(broadcast, 500).unref();

server.listen(PORT, () => {
  console.log(`mock api listening on :${PORT}`);
});
