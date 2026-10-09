// Reproducible, read-only slice of GitHub's official OpenAPI description.
import fs from "node:fs";

const revision = "7dee0622aeecf9df3c5060ca28c7a57ee5007804";
const source = `https://raw.githubusercontent.com/github/rest-api-description/${revision}/descriptions/api.github.com/api.github.com.json`;
const spec = process.argv[2]
  ? JSON.parse(fs.readFileSync(process.argv[2], "utf8"))
  : await (await fetch(source)).json();
const paths = [
  "/repos/{owner}/{repo}/pulls",
  "/repos/{owner}/{repo}/pulls/{pull_number}",
  "/repos/{owner}/{repo}/pulls/{pull_number}/files",
  "/repos/{owner}/{repo}/pulls/{pull_number}/reviews",
  "/repos/{owner}/{repo}/commits/{ref}/check-runs",
  "/repos/{owner}/{repo}/commits/{ref}/status",
];
const result = {
  openapi: spec.openapi,
  info: { ...spec.info, "x-statecraft-source": source },
  servers: spec.servers,
  paths: Object.fromEntries(paths.map((path) => [path, { get: spec.paths[path].get }])),
  components: {},
};
function refs(value) {
  if (!value || typeof value !== "object") return;
  if (value.$ref) {
    const [, section, name] = value.$ref.match(/^#\/components\/([^/]+)\/(.+)$/) || [];
    if (!section) throw new Error(`Unsupported reference: ${value.$ref}`);
    result.components[section] ||= {};
    if (!result.components[section][name]) {
      result.components[section][name] = spec.components[section][name];
      refs(spec.components[section][name]);
    }
  }
  for (const child of Object.values(value)) refs(child);
}
refs(result.paths);
fs.writeFileSync("api/github/read-only.json", `${JSON.stringify(result, null, 2)}\n`);
