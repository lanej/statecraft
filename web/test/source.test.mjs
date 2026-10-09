import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
  GetSourceEvidenceResponseSchema,
  ListChangesResponseSchema,
} from "../src/gen/statecraft/v1/review_pb.ts";
import { renderSource, sourceLink } from "../src/source.ts";

test("source view labels missing plan evidence and escapes source patches", () => {
  const evidence = create(GetSourceEvidenceResponseSchema, {
    sourceChange: {
      number: 42n,
      title: "<script>unsafe</script>",
      headSha: "abc",
      url: "javascript:alert(1)",
    },
    capturedAt: "2026-10-09T12:00:00Z",
    files: [
      {
        path: "main.tf",
        patchAvailable: true,
        patch: "</code><script>unsafe</script>",
      },
      { path: "binary.dat", patchAvailable: false },
    ],
    sourceReviews: [
      {
        actor: "reviewer",
        decision: "approved",
        commitSha: "old",
        createdAt: "2026-10-09T12:00:00Z",
      },
    ],
  });
  const html = renderSource({
    list: create(ListChangesResponseSchema),
    evidence,
    selected: 42n,
    query: "",
    busy: false,
    error: "",
  });
  assert.match(html, /Plan evidence unavailable/);
  assert.match(html, /Earlier commit/);
  assert.match(html, /GitHub did not supply a patch/);
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>|href="javascript:|data-action="apply"/);
});

test("provider links cannot create script or credential-bearing URLs", () => {
  for (const url of [
    "javascript:alert(1)",
    "https://github.com.evil.test/",
    "https://user:password@github.com/x",
    "http://github.com/x",
  ]) {
    assert.equal(sourceLink(url, "source"), "source");
  }
  assert.match(
    sourceLink("https://github.com/easypost/platform-infra/pull/1", "PR"),
    /^<a /,
  );
});
