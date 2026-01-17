sparman --env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN,FORGEJO_HOST=$FORGEJO_HOST  worker stop
sparman --env FORGEJO_API_TOKEN=$FORGEJO_API_TOKEN,FORGEJO_HOST=$FORGEJO_HOST  worker start
sparman worker_ui start

