import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import {
  createDemoAPI,
  requestMessage,
  shouldRefreshReview,
} from "../src/api.ts";
import { DemoWorkflowService } from "../src/gen/statecraft/v1/review_pb.ts";

test("generated demo client retains exact 64-bit versions and command evidence", async () => {
  const version = 18446744073709551614n;
  const calls = [];
  const api = createDemoAPI(
    createRouterTransport((router) => {
      router.service(DemoWorkflowService, {
        createDemo(request) {
          calls.push(request);
          return {
            review: {
              id: "demo-isolated",
              version,
              pullRequest: 9223372036854775807n,
              demo: true,
            },
          };
        },
        getDemo(request) {
          calls.push(request);
          return { review: { id: request.reviewId, version } };
        },
        actOnDemo(request) {
          calls.push(request);
          return {
            review: {
              id: request.reviewId,
              version: request.command.expectedVersion + 1n,
            },
          };
        },
      });
    }),
  );
  const created = await api.create("ready");
  assert.equal(created.version, version);
  assert.equal(created.pullRequest, 9223372036854775807n);
  assert.equal(calls[0].scenario, "ready");
  assert.equal((await api.get(created.id)).id, created.id);
  const changed = await api.act(created.id, {
    action: "request_acceptance",
    expectedVersion: created.version,
    violationId: "violation-1",
    reason: "Reviewed",
    evidence: "Recovery drill",
  });
  assert.equal(changed.version, 18446744073709551615n);
  assert.equal(calls[2].reviewId, created.id);
  assert.equal(calls[2].command.expectedVersion, version);
  assert.equal(calls[2].command.violationId, "violation-1");
  assert.equal(calls[2].command.evidence, "Recovery drill");
});

test("stale and denied decisions refresh eligibility while preserving the server reason", async () => {
  for (const code of [Code.Aborted, Code.FailedPrecondition]) {
    const api = createDemoAPI(
      createRouterTransport((router) => {
        router.service(DemoWorkflowService, {
          actOnDemo() {
            throw new ConnectError("Current plan is required", code);
          },
        });
      }),
    );
    await assert.rejects(
      api.act("demo-1", { action: "approve", expectedVersion: 1n }),
      (error) => {
        assert.equal(shouldRefreshReview(error), true);
        assert.equal(requestMessage(error), "Current plan is required");
        return true;
      },
    );
  }
  for (const code of [
    Code.InvalidArgument,
    Code.NotFound,
    Code.Unavailable,
    Code.Internal,
  ]) {
    assert.equal(shouldRefreshReview(new ConnectError("error", code)), false);
  }
});

test("missing review payload fails without inventing a successful action", async () => {
  const api = createDemoAPI(
    createRouterTransport((router) => {
      router.service(DemoWorkflowService, {
        createDemo() {
          return {};
        },
      });
    }),
  );
  await assert.rejects(
    api.create("ready"),
    (error) => error.code === Code.Internal,
  );
});
