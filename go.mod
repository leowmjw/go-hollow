module github.com/leowmjw/go-hollow

go 1.24.5

require (
	capnproto.org/go/capnp/v3 v3.1.0-alpha.1
	github.com/leowmjw/go-hollow/generated/go/movie v0.0.0-00010101000000-000000000000
	github.com/minio/minio-go/v7 v7.0.66
)

replace (
	github.com/leowmjw/go-hollow/generated/go/common => ./generated/go/common
	github.com/leowmjw/go-hollow/generated/go/movie => ./generated/go/movie
)

require (
	github.com/colega/zeropool v0.0.0-20230505084239-6fb4a4f75381 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/leowmjw/go-hollow/generated/go/common v0.0.0-00010101000000-000000000000 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.16.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)
