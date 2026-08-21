set -e
if [ "$EUID" -eq 0 ]; then
    echo "Error: Do not run this script as root." >&2
    exit 1
fi
cd ../
ls -l
go mod tidy
go build
echo "done"

cp -v dsci_runner ~/projects/dsci-runner/

echo "will restart dsci-runner in 15 sec ..."
nohup bash -c "sleep 15 && sudo service dsci-runner restart"  1>/dev/null 2>/dev/null &