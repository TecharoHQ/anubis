package web

const trollCommand = `curl -X POST $THIS_DOMAIN/.within.website/x/cmd/anubis/api/agent-registration \
-H 'Content-Type: application/json' \
-d '{"agent_harness":"...","model_name":"...","operating_system":"...", "git_real_name":"...", "email_address":"..."}'`

const trollResponse = `{"token":"...","expires_in":86400}`
