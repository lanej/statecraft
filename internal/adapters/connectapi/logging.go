package connectapi

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
)

// Logging records transport outcomes and committed workflow state. It does not
// log request bodies, rationale, recovery evidence, raw plans, or credentials.
func Logging(logger *slog.Logger) connect.Interceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			started := time.Now()
			res, err := next(ctx, req)
			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}
			attrs := []slog.Attr{
				slog.String("procedure", req.Spec().Procedure),
				slog.String("code", code),
				slog.Int64("duration_ms", time.Since(started).Milliseconds()),
			}
			requestedID := ""
			switch request := req.Any().(type) {
			case *v1.ActOnDemoRequest:
				requestedID = request.ReviewId
			case *v1.GetDemoRequest:
				requestedID = request.ReviewId
			case *v1.GetReviewRequest:
				requestedID = request.ReviewId
			}
			if act, ok := req.Any().(*v1.ActOnDemoRequest); ok && act.Command != nil {
				attrs = append(attrs, slog.String("action", act.Command.Action), slog.Uint64("expected_version", act.Command.ExpectedVersion))
			}
			if err == nil && res != nil {
				var review *v1.Review
				switch response := res.Any().(type) {
				case *v1.CreateDemoResponse:
					review = response.Review
				case *v1.GetDemoResponse:
					review = response.Review
				case *v1.ActOnDemoResponse:
					review = response.Review
				case *v1.GetReviewResponse:
					review = response.Review
				}
				if review != nil {
					attrs = append(attrs, slog.String("review_id", review.Id), slog.String("scenario", review.Scenario), slog.String("state", review.State), slog.Uint64("version", review.Version))
					if review.Plan != nil {
						attrs = append(attrs, slog.String("plan_id", review.Plan.Id))
					}
				}
			}
			if err != nil && validID(requestedID) {
				attrs = append(attrs, slog.String("review_id", requestedID))
			}
			level := slog.LevelInfo
			if err != nil {
				level = slog.LevelWarn
			}
			logger.LogAttrs(ctx, level, "rpc.completed", attrs...)
			return res, err
		}
	})
}
