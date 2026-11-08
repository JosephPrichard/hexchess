module hexchess-svc

go 1.27

require (
	github.com/aws/aws-sdk-go-v2 v1.47.8
	github.com/aws/aws-sdk-go-v2/config v1.33.16
	github.com/aws/aws-sdk-go-v2/credentials v1.20.16
	github.com/aws/aws-sdk-go-v2/feature/rds/auth v1.6.31
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.9
	github.com/brianvoe/gofakeit v3.18.0+incompatible
	github.com/elastic/go-elasticsearch/v8 v8.19.7
	github.com/go-chi/chi/v5 v5.3.1
	github.com/go-playground/locales v0.14.1
	github.com/go-playground/universal-translator v0.18.1
	github.com/go-playground/validator/v10 v10.30.3
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	github.com/hellofresh/health-go/v5 v5.5.5
	github.com/jackc/pgx/v5 v5.11.0
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10
	github.com/redis/go-redis/v9 v9.23.0
	github.com/riverqueue/river v0.48.0
	github.com/riverqueue/river/riverdriver/riverpgxv5 v0.48.0
	github.com/riverqueue/river/rivertype v0.48.0
	github.com/stretchr/testify v1.12.1
	github.com/thomaspoignant/go-feature-flag v1.58.0
	go.opentelemetry.io/contrib/bridges/otelslog v0.21.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.23.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.uber.org/atomic v1.12.0
	go.uber.org/mock v0.6.0
	golang.org/x/crypto v0.58.0
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba
	golang.org/x/sync v0.24.0
	google.golang.org/api v0.308.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	riverqueue.com/riverui v0.21.0
)

require (
	cloud.google.com/go/auth v0.24.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.3.0 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.26 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.8 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager v0.5.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.9 // indirect
	github.com/aws/smithy-go v1.28.8 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/diegoholiveira/jsonlogic/v3 v3.10.1 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.9.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.11 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.23 // indirect
	github.com/googleapis/gax-go/v2 v2.26.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20250907135507-afb5586c32a6 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/luci/go-render v0.0.0-20160219211803-9a04cc21af0f // indirect
	github.com/nikunjy/rules v1.5.0 // indirect
	github.com/r3labs/diff/v3 v3.0.2 // indirect
	github.com/riverqueue/apiframe v0.0.0-20251229202423-2b52ce1c482e // indirect
	github.com/riverqueue/river/riverdriver v0.48.0 // indirect
	github.com/riverqueue/river/rivershared v0.48.0 // indirect
	github.com/thomaspoignant/go-feature-flag/modules/core v0.8.0 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.2 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
