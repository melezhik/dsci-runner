set -e

echo "start build ..."

pwd

ls -l

2>&1

mkdir -p ~/.dsci/.sparky

chmod a+w ~/.dsci

chmod a+w ~/.dsci/.sparky/

if [[ $OSTYPE == darwin* ]]; then
    gid=1001
else
    gid=$(id -g)
fi

docker build . --build-arg UID=$(id -u) --build-arg GID=$gid -t dsci-dispatch

docker stop -t 1 dsci-dispatch || :


docker run \
-id \
--rm --name dsci-dispatch \
--network host \
-v /var/run/docker.sock:/var/run/docker.sock \
-v $HOME/.dsci/.sparky:/home/worker/.sparky:rw \
dsci-dispatch || :

docker logs --tail 100 dsci-dispatch
