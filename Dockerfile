FROM alpine:latest

ENV PATH="/home/worker/.raku/bin:/opt/rakudo-pkg/bin:${PATH}"

ENV SP6_DUMP_TASK_CODE=0

ENV SP6_FORMAT_COLOR=1

RUN apk update && apk add openssl bash curl wget perl openssl-dev sudo git

RUN apk add --no-cache bash

RUN curl -1sLf \
  'https://dl.cloudsmith.io/public/nxadm-pkgs/rakudo-pkg/setup.alpine.sh' \
  | bash 

RUN apk add rakudo-pkg

RUN adduser -D -h /home/worker -s /bin/bash -G wheel worker

RUN echo '%wheel ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers

RUN addgroup worker wheel

RUN sudo echo

USER worker

RUN git clone https://github.com/ugexe/zef.git /tmp/zef && \

cd /tmp/zef && raku -I. bin/zef install . --/test --install-to=home

RUN zef install --/test JSON::Fast --debug

RUN sudo apk add build-base

RUN echo OK4 && zef install --/test https://github.com/melezhik/Sparrow6.git

RUN echo OK7 && zef install --/test --force-install https://github.com/melezhik/sparky-job-api.git

RUN sudo apk add go --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community

RUN zef install --/test Cro::TLS

RUN zef install --/test Sparrowdo

RUN echo OK && zef install --/test --force-install https://github.com/melezhik/sparky.git

RUN sudo apk add sqlite-libs openssh-keygen util-linux openssh-client python3

RUN ssh-keygen -t ed25519 -f ~/.ssh/id_rsa -q -N ""

RUN echo OK2 && mkdir /home/worker/projects && cd /home/worker/projects && git clone https://github.com/melezhik/sparky.git && cd /home/worker/projects/sparky && zef install . --force-install --/test

RUN cd /home/worker/projects/sparky && raku db-init.raku

ENTRYPOINT cd /home/worker/projects/sparky && sparman --base $PWD worker_ui conf &&  sparman worker_ui start  && sparman --env SPARKY_TIMEOUT=10 worker start && tail -f ~/.sparky/sparkyd.log

COPY sparrowfile sparky.yaml /home/worker/.sparky/projects/dsci/

RUN sudo chown -R worker /home/worker/.sparky/ /home/worker/projects/

WORKDIR /home/worker/projects

ENV FORGEJO_HOST=http://host.docker.internal:3000

ENV FORGEJO_API_TOKEN=changeme

EXPOSE 4000
