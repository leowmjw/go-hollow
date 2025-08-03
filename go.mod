module github.com/leowmjw/go-hollow

go 1.24.5

require (
	capnproto.org/go/capnp/v3 v3.1.0-alpha.1
	github.com/leowmjw/go-hollow/generated/go/delta v0.0.0-00010101000000-000000000000
	github.com/leowmjw/go-hollow/generated/go/movie v0.0.0-00010101000000-000000000000
	github.com/minio/minio-go/v7 v7.0.95
)

replace (
	github.com/leowmjw/go-hollow/generated/go/common => ./generated/go/common
	github.com/leowmjw/go-hollow/generated/go/delta => ./generated/go/delta
	github.com/leowmjw/go-hollow/generated/go/movie => ./generated/go/movie
)

require (
	github.com/colega/zeropool v0.0.0-20230505084239-6fb4a4f75381 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leowmjw/go-hollow/generated/go/common v0.0.0-00010101000000-000000000000 // indirect
	github.com/minio/crc64nvme v1.1.0 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/tinylib/msgp v1.3.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
)
