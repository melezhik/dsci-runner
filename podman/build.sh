set -e

echo "start build ..."

exit

pwd

ls -l

2>&1

mkdir -p ~/.dsci/.sparky

chmod a+w ~/.dsci

chmod a+w ~/.dsci/.sparky/

podman build . -t dsci-dispatch

podman stop -t 1 dsci-dispatch || :

opts=""

if test -d ~/.dsci/.secrets; then
    echo "mount secrets from $HOME/.dsci/.secrets"
    opts="$opts -v $HOME/.dsci/.secrets:/root/.secrets"
fi

podman rm -f dsci-dispatch  || :

podman container cleanup --all

podman run \
-id \
--name dsci-dispatch \
--network host \
--privileged \
-v $HOME/.dsci/.sparky:/root/.sparky:rw,Z,U \
-e HOST_SSH_USER=$USER \
$opts \
dsci-dispatch

podman cp dsci-dispatch:/root/.ssh/id_rsa.pub /tmp/

k=$(cat /tmp/id_rsa.pub)

if grep -q "$k"  ~/.ssh/authorized_keys; then
    echo "dsci orchestartor public key exists at host ~/.ssh/authorized_keys"
else
    echo "insert dsci orchestartor public key to host ~/.ssh/authorized_keys"
    echo $k >> ~/.ssh/authorized_keys
fi

echo "run job scheduller ..."

