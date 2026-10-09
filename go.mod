module github.com/lanej/statecraft

go 1.26.0

require (
	connectrpc.com/connect v1.21.0
	github.com/google/go-github/v92 v92.0.0
	google.golang.org/protobuf v1.36.11
)

require github.com/google/go-querystring v1.2.0 // indirect

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)
