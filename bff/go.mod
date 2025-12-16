module github.com/daisuke8000/example-ec-platform/bff

go 1.25

require (
	connectrpc.com/connect v1.16.2
	github.com/lestrrat-go/jwx/v2 v2.0.21
	github.com/redis/go-redis/v9 v9.5.1
	github.com/rs/cors v1.11.0
	github.com/daisuke8000/example-ec-platform/gen v0.0.0
	github.com/sethvargo/go-envconfig v1.0.3
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/trace v1.27.0
	golang.org/x/net v0.25.0
	google.golang.org/grpc v1.64.0
)

replace github.com/daisuke8000/example-ec-platform/gen => ../gen
