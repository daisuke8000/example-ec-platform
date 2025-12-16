module github.com/daisuke8000/example-ec-platform/services/user

go 1.25

require (
	connectrpc.com/connect v1.18.1
	github.com/daisuke8000/example-ec-platform/gen v0.0.0
	github.com/daisuke8000/example-ec-platform/pkg/connect v0.0.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/redis/go-redis/v9 v9.17.2
	github.com/sethvargo/go-envconfig v1.0.3
	golang.org/x/crypto v0.32.0
	golang.org/x/net v0.25.0
	google.golang.org/protobuf v1.35.2
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/grpc v1.64.0 // indirect
)

replace (
	github.com/daisuke8000/example-ec-platform/gen => ../../gen
	github.com/daisuke8000/example-ec-platform/pkg/connect => ../../pkg/connect
)
