bind {
  http    = ":3004"
  https   = ":3005"
  metrics = ":9091"
}

domain "osiris.local.cetacean.club" {
  tls {
    cert = "./internal/config/testdata/tls/selfsigned.crt"
    key  = "./internal/config/testdata/tls/selfsigned.key"
  }

  target        = "http://localhost:3000"
  health_target = "http://localhost:9091/healthz"
}