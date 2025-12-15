module github.com/sasakidaisuke/example-ec-platform/services/product

go 1.25

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/sasakidaisuke/example-ec-platform/gen v0.0.0
	github.com/sethvargo/go-envconfig v1.0.3
	google.golang.org/grpc v1.64.0
)

replace github.com/sasakidaisuke/example-ec-platform/gen => ../../gen
