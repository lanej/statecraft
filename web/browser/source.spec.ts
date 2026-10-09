import { create, toJson } from "@bufbuild/protobuf";
import { expect, type Page, test } from "@playwright/test";
import {
  GetSourceEvidenceResponseSchema,
  ListChangesResponseSchema,
} from "../src/gen/statecraft/v1/review_pb";

// Synthetic API responses isolate UI regressions. The live runtime always reads
// GitHub; these fixtures never enter the application or deployment image.
async function sourcePage(page: Page) {
  let unavailable = false;
  const list = create(ListChangesResponseSchema, {
    repository: "easypost/platform-infra",
    capturedAt: "2026-10-09T12:00:00Z",
    sourceChanges: [
      { number: 12n, title: "Change network", author: "reviewer" },
    ],
  });
  const evidence = create(GetSourceEvidenceResponseSchema, {
    sourceChange: {
      number: 12n,
      title: "Change network",
      headSha: "a".repeat(40),
    },
    capturedAt: "2026-10-09T12:00:00Z",
    files: [
      {
        path: `live/${"long-resource-name/".repeat(12)}main.tf`,
        patch: `+ ${"source ".repeat(100)}`,
        patchAvailable: true,
      },
    ],
  });
  await page.route("**/runtime", (route) =>
    route.fulfill({ json: { mode: "read_only" } }),
  );
  await page.route(
    "**/statecraft.v1.SourceEvidenceService/ListChanges",
    (route) => route.fulfill({ json: toJson(ListChangesResponseSchema, list) }),
  );
  await page.route(
    "**/statecraft.v1.SourceEvidenceService/GetSourceEvidence",
    (route) =>
      unavailable
        ? route.fulfill({
            status: 503,
            json: { code: "unavailable", message: "Evidence unavailable" },
          })
        : route.fulfill({
            json: toJson(GetSourceEvidenceResponseSchema, evidence),
          }),
  );
  await page.goto("/?pr=12");
  await expect(
    page.getByRole("heading", { name: "Change network" }),
  ).toBeVisible();
  return (failed: boolean) => {
    unavailable = failed;
  };
}

test("refresh preserves disclosure and focus, labels old evidence on failure, and recovers", async ({
  page,
}) => {
  const fail = await sourcePage(page);
  const file = page.locator(".source-file");
  await file.locator("summary").click();
  await expect(file).toHaveAttribute("open", "");
  const refresh = page.getByRole("button", { name: "Refresh evidence" });
  await refresh.click();
  await expect(page.locator(".source-workspace")).toHaveAttribute(
    "aria-busy",
    "false",
  );
  await expect(file).toHaveAttribute("open", "");
  await expect(refresh).toBeFocused();

  fail(true);
  await refresh.click();
  await expect(page.getByRole("alert")).toContainText("previous capture");
  await expect(
    page.getByRole("heading", { name: "Change network" }),
  ).toBeVisible();
  fail(false);
  await refresh.click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.locator(".source-workspace")).toHaveAttribute(
    "aria-busy",
    "false",
  );
});

test("filtering retains input focus and long source patches do not widen the narrow layout", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await sourcePage(page);
  const search = page.getByLabel("Find a pull request");
  await search.fill("missing");
  await expect(page.locator(".source-pr")).toHaveCount(0);
  await expect(search).toBeFocused();
  await search.fill("network");
  await expect(page.locator(".source-pr")).toHaveCount(1);
  await page.locator(".source-file summary").click();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await expect(
    page.getByRole("heading", { name: "Plan evidence unavailable" }),
  ).toHaveCount(0);
  await expect(page.locator(".source-note")).toContainText(
    "Plan evidence unavailable",
  );
  await expect(page.locator("[data-action]")).toHaveCount(0);
});
