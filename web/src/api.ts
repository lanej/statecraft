import type { MessageInitShape } from "@bufbuild/protobuf";
import {
  Code,
  ConnectError,
  createClient,
  type Transport,
} from "@connectrpc/connect";
import {
  DemoWorkflowService,
  type Review,
  type WorkflowCommandSchema,
} from "./gen/statecraft/v1/review_pb.ts";

export type DemoCommand = MessageInitShape<typeof WorkflowCommandSchema>;

function requireReview(response: { review?: Review }): Review {
  if (!response.review)
    throw new ConnectError(
      "The demo response did not contain a review.",
      Code.Internal,
    );
  return response.review;
}

// All wire encoding, methods and message types come from the generated schema.
export function createDemoAPI(transport: Transport) {
  const client = createClient(DemoWorkflowService, transport);
  const options = { timeoutMs: 30_000 };
  return {
    async create(scenario: string): Promise<Review> {
      return requireReview(await client.createDemo({ scenario }, options));
    },
    async get(reviewId: string): Promise<Review> {
      return requireReview(await client.getDemo({ reviewId }, options));
    },
    async act(reviewId: string, command: DemoCommand): Promise<Review> {
      return requireReview(
        await client.actOnDemo({ reviewId, command }, options),
      );
    },
  };
}

export const requestMessage = (error: unknown): string =>
  ConnectError.from(error).rawMessage;

export const shouldRefreshReview = (error: unknown): boolean =>
  [Code.Aborted, Code.FailedPrecondition].includes(
    ConnectError.from(error).code,
  );
