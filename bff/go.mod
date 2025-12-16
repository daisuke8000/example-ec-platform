module github.com/daisuke8000/example-ec-platform/bff

go 1.25

require (
	connectrpc.com/connect v1.18.1
	github.com/lestrrat-go/jwx/v2 v2.1.6
	github.com/sethvargo/go-envconfig v1.0.3
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/lestrrat-go/blackmagic v1.0.3 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.6 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
)

replace github.com/daisuke8000/example-ec-platform/gen => ../gen
