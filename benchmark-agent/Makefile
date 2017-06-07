GLIDE=$(which glide)
GO_EXECUTABLE ?= go
PACKAGES=$(glide novendor)
IMAGE=hyperpilot/benchmark-agent
TAG=latest

init:
	${GLIDE} install

test:
	${GO_EXECUTABLE} test ${PACKAGES}

build:
	CGO_ENABLED=0 go build -a -installsuffix cgo

build-docker:
	sudo docker build . -t ${IMAGE}:${TAG} 

push:
	sudo docker push ${IMAGE}:${TAG}

run:
	sudo docker run -v /var/run/docker.sock:/var/run/docker.sock -p 7778:7778 -d ${IMAGE}:${TAG}

clean:
	rm -rf vendor
