docker run \
--rm --name dsci \
--port 3000:3000,4000:4000 \
--env FORGEJO_HOST=$FORGEJO_HOST \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN \
dsci
