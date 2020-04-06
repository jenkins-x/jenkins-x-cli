FROM centos:7

RUN yum install -y git

ENTRYPOINT ["jx", "version"]

COPY ./build/linux/jx /usr/bin/jx