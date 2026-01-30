set -e

echo "start build ..."
pwd
ls -l

2>&1

docker network create dsci || :

docker build . -t dsci-dispatch

docker stop -t 1 dsci-dispatch || :

docker run \
-td \
--network dsci \
--rm --name dsci-dispatch -it \
--env FORGEJO_HOST=$FORGEJO_HOST \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN \
-v /var/run/docker.sock:/var/run/docker.sock \
-v $HOME/.sparky:/home/worker/.sparky \
dsci-dispatch

sleep 10

docker logs --tail 100 dsci-dispatch