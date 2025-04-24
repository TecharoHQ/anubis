module github.com/TecharoHQ/anubis/test

go 1.24.2

replace github.com/TecharoHQ/anubis => ..

require (
	github.com/facebookgo/flagenv v0.0.0-20160425205200-fcd59fca7456
	github.com/google/uuid v1.6.0
)

require (
	github.com/TecharoHQ/anubis v1.16.0 // indirect
	github.com/a-h/templ v0.3.857 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/facebookgo/ensure v0.0.0-20200202191622-63f1cf65ac4c // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20200203212716-c811ad88dec4 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/jsha/minica v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sebest/xff v0.0.0-20210106013422-671bd2870b3a // indirect
	github.com/yl2chen/cidranger v1.0.2 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

tool (
	github.com/TecharoHQ/anubis/cmd/anubis
	github.com/jsha/minica
)
