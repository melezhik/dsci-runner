curl \
-X POST  \
-H 'Content-Type: application/json' \
--data @job_req.json 127.0.0.1:8080/queue \
-D -
