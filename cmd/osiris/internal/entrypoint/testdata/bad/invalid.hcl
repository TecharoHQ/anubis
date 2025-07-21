bind {
  http    = ":65530"
  https   = ":65531"
  metrics = ":65532"
}

domain "osiris.local.cetacean.club" {
  tls {
    cert = "./testdata/invalid.crt"
    key  = "./testdata/invalid.key"
  }

  target        = "http://localhost:3000"
  health_target = "http://localhost:9091/healthz"
}