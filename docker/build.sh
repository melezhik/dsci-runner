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

opts=""

if test -d ~/.dsci/.secrets; then
    echo "mount secrets from $HOME/.dsci/.secrets"
    opts="-v $HOME/.dsci/.secrets:/home/worker/.secrets"
fi

docker run \
-id \
--rm --name dsci-dispatch \
--network host \
-v /var/run/docker.sock:/var/run/docker.sock \
-v $HOME/.dsci/.sparky:/home/worker/.sparky:rw \
-e HOST_SSH_USER=$USER \
$opts \
dsci-dispatch || :

docker cp dsci-dispatch:/home/worker/.ssh/id_rsa.pub /tmp/

k=$(cat /tmp/id_rsa.pub)

if grep -q "$k"  ~/.ssh/authorized_keys; then
    echo "dsci orchestartor public key exists at host ~/.ssh/authorized_keys"
else
    echo "insert dsci orchestartor public key to host ~/.ssh/authorized_keys"
    echo $k >> ~/.ssh/authorized_keys
fi

docker logs --tail 100 dsci-dispatch
