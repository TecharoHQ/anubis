# /etc/nginx/conf-anubis.inc

# Forward to anubis
location / {
  proxy_set_header Host $host;
  proxy_set_header X-Real-IP $remote_addr;
  proxy_pass http://anubis;
}