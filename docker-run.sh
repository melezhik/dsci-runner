docker network create dsci || :
docker run \
--network dsci \
--rm --name dsci -it \
-p 4000:4000 \
--env FORGEJO_HOST=$FORGEJO_HOST \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN \
-v /var/run/docker.sock:/var/run/docker.sock \
dsci
