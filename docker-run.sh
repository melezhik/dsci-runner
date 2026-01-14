docker run \
--rm --name dsci -d \
-p 3333:3333 -p 4000:4000 \
--env FORGEJO_HOST=$FORGEJO_HOST \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN \
dsci
