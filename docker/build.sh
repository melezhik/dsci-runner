set -e

echo "start build ..."

pwd

ls -l

2>&1

mkdir -p ~/.dsci

chmod a+w ~/.dsci

docker network create dsci || :

docker build . --build-arg UID=$(id -u) --build-arg GID=$(id -g) -t dsci-dispatch

docker stop -t 1 dsci-dispatch || :

docker run \
-td \
--network dsci \
--rm --name dsci-dispatch -it \
--env FORGEJO_HOST=$FORGEJO_HOST \
--env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN \
-v /var/run/docker.sock:/var/run/docker.sock \
-v $HOME/.dsci/.sparky:/home/worker/.sparky \
dsci-dispatch

sleep 10

docker logs --tail 100 dsci-dispatch
