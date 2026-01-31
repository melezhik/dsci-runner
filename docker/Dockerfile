FROM alpine:latest

ENV PATH="/home/worker/.raku/bin:${PATH}"

ENV SP6_DUMP_TASK_CODE=0

ENV SP6_FORMAT_COLOR=1

RUN apk update && apk add openssl bash curl wget perl openssl-dev sudo git docker-cli

RUN apk add --no-cache bash

RUN apk add rakudo zef

RUN sudo echo

ARG UID=1000
ARG GID=1000

RUN addgroup -g $GID -S worker && \
    adduser -u $UID -S worker -G worker -h /home/worker -s /bin/bash

RUN chown -R worker:worker /home/worker

RUN addgroup worker wheel 

RUN echo '%wheel ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers

USER worker

RUN sudo apk add build-base sqlite-libs openssh-keygen util-linux openssh-client python3

RUN echo OK4 && zef install --/test https://github.com/melezhik/Sparrow6.git

RUN echo OK7 && zef install --/test --force-install https://github.com/melezhik/sparky-job-api.git

RUN sudo apk add go --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community

RUN zef install --/test JSON::Fast Sparrowdo YAMLish DBIish DBIish::Pool JSON::Fast  Time::Crontab File::Directory::Tree Data::Dump

#RUN ssh-keygen -t ed25519 -f ~/.ssh/id_rsa -q -N ""

RUN echo OK2 && mkdir -p /home/worker/projects && cd /home/worker/projects && git clone https://github.com/melezhik/sparky.git && cd sparky && zef install --/test --/depends . 

ENTRYPOINT cd /home/worker/projects/sparky; if test -f ~/.sparky/projects/db.sqlite3; then echo "db exist"; else raku db-init.raku; fi; mkdir -p ~/.sparky/projects/dsci; cp /home/worker/distro/sparrowfile ~/.sparky/projects/dsci/; cp /home/worker/distro/sparky.yaml ~/sparky.yaml; sparkyd

COPY sparrowfile sparky.yaml /home/worker/distro/

RUN sudo chown -R worker /home/worker/projects/ /home/worker/distro

RUN git config --global advice.detachedHead false

WORKDIR /home/worker/projects

ENV FORGEJO_HOST=http://127.0.0.1:3000

ENV FORGEJO_API_TOKEN=changeme

ENV DSCI_FEEDBACK_URL=http:/127.0.0.1:4000

ENV DSCI_AGENT_IMAGE=alpine:latest

