import { mkdtemp, readFile, writeFile } from "node:fs/promises"
import { createServer } from "node:http"
import { tmpdir } from "node:os"
import { extname, join } from "node:path"

const root = new URL("../../dist/", import.meta.url).pathname.replace(
  /^\/(.:\/)/,
  "$1",
)
const initialState = JSON.parse(
  await readFile(new URL("./management.json", import.meta.url), "utf8"),
)
const state = structuredClone(initialState)
const stateDir = await mkdtemp(join(tmpdir(), "facet-studio-ui-fixture-"))
const statePath = join(stateDir, "management.json")
const persist = () => writeFile(statePath, JSON.stringify(state, null, 2))
await persist()
const json = (res, body) => {
  res.writeHead(200, { "content-type": "application/json" })
  res.end(JSON.stringify(body))
}
createServer(async (req, res) => {
  if (req.url === "/__fixture/reset" && req.method === "POST") {
    Object.assign(state, structuredClone(initialState))
    await persist()
    return json(res, { status: "ok" })
  }
  if (req.url === "/api/provider-instances")
    return json(res, { instances: state.instances })
  if (req.url === "/api/provider-targets")
    return json(res, { targets: state.targets })
  if (req.url === "/api/provider-instances/catalogs")
    return json(res, { catalogs: state.catalogs })
  if (req.url === "/api/model-routes" && req.method === "GET")
    return json(res, { routes: state.routes })
  if (req.url === "/api/provider-roster")
    return json(res, {
      providers: state.providers,
    })
  if (req.url?.endsWith("/catalog/sync")) {
    let body = ""
    for await (const chunk of req) body += chunk
    if (body !== "{}") {
      res.writeHead(400)
      return res.end("sync body must be empty")
    }
    return json(res, { models: [] })
  }
  if (req.url?.startsWith("/api/model-routes/")) {
    if (req.method === "PUT") {
      let body = ""
      for await (const chunk of req) body += chunk
      const route = JSON.parse(body)
      state.routes = state.routes.map((item) =>
        item.name === route.name ? route : item,
      )
      await persist()
    }
    return json(res, { status: "ok" })
  }
  const path =
    req.url === "/" || req.url === "/models" ? "index.html" : req.url.slice(1)
  try {
    const data = await readFile(join(root, path))
    const types = {
      ".html": "text/html",
      ".js": "text/javascript",
      ".css": "text/css",
    }
    res.writeHead(200, {
      "content-type": types[extname(path)] || "application/octet-stream",
    })
    res.end(data)
  } catch {
    const data = await readFile(join(root, "index.html"))
    res.writeHead(200, { "content-type": "text/html" })
    res.end(data)
  }
}).listen(4178, "127.0.0.1")
