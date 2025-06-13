module github.com/TecharoHQ/anubis/test

go 1.24.2

replace github.com/TecharoHQ/anubis => ..

require (
	github.com/TecharoHQ/anubis v1.18.0
	github.com/facebookgo/flagenv v0.0.0-20160425205200-fcd59fca7456
	github.com/google/uuid v1.6.0
)

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/a-h/templ v0.3.898 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/facebookgo/ensure v0.0.0-20200202191622-63f1cf65ac4c // indirect
	github.com/facebookgo/subset v0.0.0-20200203212716-c811ad88dec4 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/cel-go v0.25.0 // indirect
	github.com/jsha/minica v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.64.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sebest/xff v0.0.0-20210106013422-671bd2870b3a // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/yl2chen/cidranger v1.0.2 // indirect
	golang.org/x/exp v0.0.0-20250506013437-ce4c2cf36ca6 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250519155744-55703ea1f237 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250519155744-55703ea1f237 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	k8s.io/apimachinery v0.33.1 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

tool (
	github.com/TecharoHQ/anubis/cmd/anubis
	github.com/jsha/minica
)
