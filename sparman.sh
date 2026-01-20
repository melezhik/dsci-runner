sparman worker stop
sparman \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN,\
FORGEJO_HOST=$FORGEJO_HOST,\
DSCI_AGENT_SKIP_BOOTSTRAP=1,\
DSCI_AGENT_IMAGE=dsci-agent-alpine:latest \
worker start
sparman worker_ui start

