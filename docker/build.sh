set -e

echo "start build ..."

pwd

ls -l

2>&1

mkdir -p ~/.dsci/.sparky

chmod a+w ~/.dsci

chmod a+w ~/.dsci/.sparky/

cp ~/.dsci.toml .

docker network create dsci || :

docker build . --build-arg UID=$(id -u) --build-arg GID=$(id -g) -t dsci-dispatch

docker stop -t 1 dsci-dispatch || :

docker run \
-id \
--network dsci \
--rm --name dsci-dispatch \
--add-host host.docker.internal:host-gateway \
-v /var/run/docker.sock:/var/run/docker.sock \
-v $HOME/.dsci/.sparky:/home/worker/.sparky:rw \
dsci-dispatch

sleep 10

docker logs --tail 100 dsci-dispatch
