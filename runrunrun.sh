#!/bin/bash

for i in {1..500} ; do
    docker run -it --rm --init --volume $PWD:/workdir --env BUILDKITE_BUILD_ID=$i --volume /var/run/docker.sock:/var/run/docker.sock --workdir /workdir docker:23.0.0 /bin/sh -e -c $'apk add g++ make bash gcompat curl mysql mysql-client\nwget https://golang.org/dl/go1.19.4.linux-amd64.tar.gz\ntar -C /usr/local -xzf go1.19.4.linux-amd64.tar.gz\nrm go1.19.4.linux-amd64.tar.gz\nexport PATH=/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/usr/bin:/usr/local/go/bin && make upgrade-test'
    if [ $? -eq 0 ]; then
      echo "Attempt #$i success" >> runrunrun_log.txt
      docker container stop kind-$i-control-plane
      docker container rm kind-$i-control-plane
    else
      echo "========================" >> runrunrun_log.txt
      echo "Attempt #$i failed" >> runrunrun_log.txt
    fi
done

