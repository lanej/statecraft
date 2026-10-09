import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  type GetSourceEvidenceResponse,
  type ListChangesResponse,
  SourceEvidenceService,
} from "./gen/statecraft/v1/review_pb.ts";
import { brandMarkURL, escapeHTML as e } from "./review.ts";

export type SourceUI = {
  list: ListChangesResponse | null;
  evidence: GetSourceEvidenceResponse | null;
  selected: bigint | null;
  query: string;
  busy: boolean;
  error: string;
};

// Source URLs are untrusted provider data. Restrict evidence links to the
// configured source host and platform-infra's existing Atlantis host.
export function sourceLink(url: string, label: string): string {
  try {
    const parsed = new URL(url);
    if (
      parsed.protocol === "https:" &&
      ["github.com", "atlantis.easypo.net"].includes(parsed.hostname) &&
      !parsed.port &&
      !parsed.username &&
      !parsed.password
    )
      return `<a href="${e(parsed.href)}" target="_blank" rel="noopener noreferrer">${e(label)}</a>`;
  } catch {
    /* Missing or malformed source links remain plain labels. */
  }
  return e(label);
}

export function renderSource(ui: SourceUI): string {
  const changes =
    ui.list?.sourceChanges.filter((change) =>
      `${change.number} ${change.title} ${change.author}`
        .toLowerCase()
        .includes(ui.query.toLowerCase()),
    ) ?? [];
  const evidence = ui.evidence;
  const change = evidence?.sourceChange;
  return `
    <header class="source-header">
      <div class="source-brand"><img src="${brandMarkURL}" alt="" width="38" height="38"><div><strong>Statecraft</strong><span>Infrastructure change, understood.</span></div></div>
      <span class="source-mode">Read only</span>
      <button class="button" data-source="refresh" ${ui.busy ? "disabled" : ""}>Refresh evidence</button>
    </header>
    <div class="source-workspace" aria-busy="${ui.busy}">
      <aside class="source-list" aria-label="Platform infrastructure pull requests">
        <h1>Platform infrastructure</h1>
        <p class="source-muted">easypost/platform-infra · Open pull requests</p>
        <label for="source-search">Find a pull request</label>
        <input id="source-search" type="search" value="${e(ui.query)}" placeholder="Title, number, or author">
        ${ui.list?.truncated ? '<p class="source-note">Showing the 500 most recently updated PRs. The list is incomplete.</p>' : ""}
        ${ui.list ? `<p class="source-muted">${changes.length} shown · Captured ${e(new Date(ui.list.capturedAt).toLocaleString())}</p>` : ""}
        <nav aria-label="Pull requests">${changes.map((pr) => `<button class="source-pr ${ui.selected === pr.number ? "selected" : ""}" data-source="select" data-number="${pr.number}" ${ui.busy ? "disabled" : ""} ${ui.selected === pr.number ? 'aria-current="true"' : ""}><span>#${pr.number} · ${e(pr.author)}${pr.draft ? " · Draft" : ""}</span><strong>${e(pr.title)}</strong></button>`).join("")}</nav>
        ${ui.list && !changes.length ? "<p>No matching open pull requests.</p>" : ""}
      </aside>
      <section class="source-detail" aria-label="Source evidence">
        ${ui.error ? `<div class="source-error" role="alert"><strong>Evidence could not be refreshed</strong><p>${e(ui.error)}</p><p>Refresh to try again. Any retained evidence is from the previous capture.</p></div>` : ""}
        ${ui.busy ? '<p role="status">Reading GitHub evidence…</p>' : ""}
        ${
          change && evidence
            ? `
          <div class="source-heading"><div><p class="source-muted">#${change.number} · ${e(change.state)}${change.draft ? " · Draft" : ""}</p><h2 tabindex="-1">${e(change.title)}</h2><p>${e(change.author)} · into ${e(change.baseRef)}</p></div>${sourceLink(change.url, "Open on GitHub ↗")}</div>
          <p class="source-muted">Head <code>${e(change.headSha)}</code> · Captured ${e(new Date(evidence.capturedAt).toLocaleString())}</p>
          <div class="source-note"><strong>Plan evidence unavailable</strong><p>This capture contains source changes and GitHub observations. Infrastructure resource changes, complete root scope, policy assessments, and resulting state have not been captured. Planning and infrastructure decisions are unavailable.</p></div>
          <section><h3>Checks and statuses on this commit</h3><p class="source-muted">Reported by GitHub. Success does not establish Statecraft approval or a verified apply.</p>${evidence.checksTruncated ? '<p class="source-note">The evidence list is incomplete; showing at most 100 check-runs and 100 commit statuses.</p>' : ""}<div class="source-checks">${evidence.checks.map((check) => `<div class="source-check"><strong>${sourceLink(check.url, check.name)}</strong><span class="source-muted">${e(check.evidenceSource)}</span><span>${e(check.status)}${check.conclusion ? ` · ${e(check.conclusion)}` : ""}</span></div>`).join("") || "<p>No check-runs or commit statuses were reported for this commit.</p>"}</div></section>
          <section><h3>Changed source files <span class="source-muted">(${evidence.files.length})</span></h3><p class="source-muted">GitHub patches may omit binary content or truncate large changes. Open GitHub for the full diff.</p>${evidence.files.map((file) => `<details class="source-file"><summary><span>${e(file.path)}</span><span class="source-muted">${e(file.status)} · +${file.additions} / −${file.deletions}</span></summary>${file.previousPath ? `<p>Previously ${e(file.previousPath)}</p>` : ""}${file.patchAvailable ? `<pre tabindex="0" aria-label="Patch for ${e(file.path)}"><code>${e(file.patch)}</code></pre>` : "<p>GitHub did not supply a patch for this file.</p>"}</details>`).join("") || "<p>No changed source files.</p>"}</section>
          <section><h3>GitHub review history</h3><p class="source-muted">External reviews remain separate from Statecraft plan approval.</p>${evidence.sourceReviews.map((review) => `<div class="source-review"><strong>${sourceLink(review.url, review.actor || "Reviewer unavailable")}</strong><span>${e(review.decision.replaceAll("_", " "))} · ${e(new Date(review.createdAt).toLocaleString())}</span><code>${e(review.commitSha || "Commit unavailable")}</code>${review.commitSha && review.commitSha !== change.headSha ? '<span class="source-muted">Earlier commit</span>' : ""}</div>`).join("") || "<p>No submitted GitHub reviews.</p>"}</section>
        `
            : !ui.busy && !ui.error
              ? '<div class="source-empty"><h2>Select a pull request</h2><p>Inspect its source changes, reviews, and checks.</p></div>'
              : ""
        }
      </section>
    </div>`;
}

export async function startSource(app: HTMLElement) {
  const client = createClient(
    SourceEvidenceService,
    createConnectTransport({ baseUrl: window.location.origin }),
  );
  const parameter = new URL(window.location.href).searchParams.get("pr");
  const selected =
    parameter &&
    /^[1-9][0-9]{0,9}$/.test(parameter) &&
    BigInt(parameter) <= 2147483647n
      ? BigInt(parameter)
      : null;
  const ui: SourceUI = {
    list: null,
    evidence: null,
    selected,
    query: "",
    busy: false,
    error: "",
  };
  const render = () => {
    const scroll = app.querySelector("nav")?.scrollTop ?? 0;
    const expanded = new Set(
      Array.from(
        app.querySelectorAll("details[open] summary span:first-child"),
        (item) => item.textContent,
      ),
    );
    app.innerHTML = renderSource(ui);
    const nav = app.querySelector("nav");
    if (nav) nav.scrollTop = scroll;
    for (const detail of app.querySelectorAll("details"))
      if (
        expanded.has(
          detail.querySelector("summary span:first-child")?.textContent ?? "",
        )
      )
        detail.open = true;
  };
  const load = async (number?: bigint) => {
    if (ui.busy) return;
    ui.busy = true;
    ui.error = "";
    if (number !== undefined) {
      ui.selected = number;
      ui.evidence = null;
      const url = new URL(window.location.href);
      url.searchParams.set("pr", number.toString());
      history.replaceState(null, "", url);
    }
    render();
    try {
      if (number === undefined) {
        ui.list = await client.listChanges({}, { timeoutMs: 30_000 });
        // A selected PR may have closed; its evidence can still be refreshed.
      }
      if (ui.selected !== null)
        ui.evidence = await client.getSourceEvidence(
          { pullRequest: ui.selected },
          { timeoutMs: 30_000 },
        );
    } catch (error) {
      ui.error =
        error instanceof Error
          ? error.message
          : "Source evidence is unavailable.";
    } finally {
      ui.busy = false;
      render();
      app
        .querySelector<HTMLElement>(
          ui.error ? "[role=alert]" : ".source-heading h2",
        )
        ?.scrollIntoView({ block: "nearest" });
      const focus = app.querySelector<HTMLElement>(
        number === undefined
          ? '[data-source="refresh"]'
          : `[data-number="${number}"]`,
      );
      (focus || app.querySelector<HTMLElement>(".source-heading h2"))?.focus({
        preventScroll: true,
      });
    }
  };
  app.addEventListener("click", (event) => {
    const target = (event.target as Element).closest<HTMLElement>(
      "[data-source]",
    );
    if (target?.dataset.source === "refresh") void load();
    if (target?.dataset.source === "select" && target.dataset.number)
      void load(BigInt(target.dataset.number));
  });
  app.addEventListener("input", (event) => {
    const input = event.target as HTMLInputElement;
    if (input.id !== "source-search") return;
    ui.query = input.value;
    const selection = [input.selectionStart, input.selectionEnd];
    render();
    const next = app.querySelector<HTMLInputElement>("#source-search");
    next?.focus();
    if (selection[0] !== null && selection[1] !== null)
      next?.setSelectionRange(selection[0], selection[1]);
  });
  await load();
}
