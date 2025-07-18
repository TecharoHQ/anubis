bind {
  http    = ":65530"
  https   = ":65531"
  metrics = ":65532"
}

domain "osiris.local.cetacean.club" {
  tls {
    cert = "./testdata/selfsigned.crt"
    key  = "./testdata/selfsigned.key"
  }

  target        = "http://localhost:3000"
  health_target = "http://localhost:9091/healthz"
}