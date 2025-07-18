bind {
  http    = ":65520"
  https   = ":65521"
  metrics = ":65522"
}

domain "http.internal" {
  tls {
    cert = "./testdata/selfsigned.crt"
    key  = "./testdata/selfsigned.key"
  }

  target        = "http://localhost:65510" # XXX(Xe) this is overwritten
  health_target = "http://localhost:9091/healthz"
}

domain "https.internal" {
  tls {
    cert = "./testdata/selfsigned.crt"
    key  = "./testdata/selfsigned.key"
  }

  target               = "https://localhost:65511" # XXX(Xe) this is overwritten
  insecure_skip_verify = true
  health_target        = "http://localhost:9091/healthz"
}

domain "h2c.internal" {
  tls {
    cert = "./testdata/selfsigned.crt"
    key  = "./testdata/selfsigned.key"
  }

  target        = "h2c://localhost:65511" # XXX(Xe) this is overwritten
  health_target = "http://localhost:9091/healthz"
}

domain "unix.internal" {
  tls {
    cert = "./testdata/selfsigned.crt"
    key  = "./testdata/selfsigned.key"
  }

  target        = "http://localhost:65511" # XXX(Xe) this is overwritten
  health_target = "http://localhost:9091/healthz"
}