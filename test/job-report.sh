project=${1:-"foo"}
job_id=${2:-"id123456"}
curl \
-s \
-X GET  \
127.0.0.1:8080/report/raw/$project/$job_id \
-D -
