FROM quay.io/podman/stable:latest

ENV PATH="/root/.raku/bin:/usr/share/perl6/site/bin:${PATH}"

ENV SP6_DUMP_TASK_CODE=0

ENV SP6_FORMAT_COLOR=1

RUN dnf install -y openssl wget git podman

RUN yum -y install rakudo

RUN git clone https://github.com/ugexe/zef.git && cd zef &&  raku -I. bin/zef install . --/test

RUN echo OK4 && zef install --/test https://github.com/melezhik/Sparrow6.git

RUN echo OK7 && zef install --/test --force-install https://github.com/melezhik/sparky-job-api.git

RUN zef install --/test JSON::Fast Sparrowdo YAMLish DBIish DBIish::Pool JSON::Fast  Time::Crontab File::Directory::Tree Data::Dump

RUN ssh-keygen -t ed25519 -f /root/.ssh/id_rsa -q -N ""

RUN echo OK8 && mkdir -p /root/projects && cd /root/projects && git clone https://github.com/melezhik/sparky.git && cd sparky && zef install --/test --/depends --force-install . 

ENTRYPOINT cd /root/projects/sparky; if test -f /root/.sparky/projects/db.sqlite3; then echo "db exist"; else raku db-init.raku; fi; mkdir -p /root/.sparky/projects/dsci; cp /root/distro/sparrowfile /root/.sparky/projects/dsci/; cp /root/distro/sparky.yaml /root/sparky.yaml; sparkyd

COPY sparrowfile sparky.yaml /root/distro/

RUN git config --global advice.detachedHead false

RUN echo OK4 && zef install --/test --force-install https://github.com/melezhik/sparky-job-api.git

WORKDIR /root/projects

ENV HOST_SSH_USER=root

ENV SPARROWDO_DOCKER_BIN=/usr/bin/podman

RUN dnf -yq install procps-ng util-linux-script
