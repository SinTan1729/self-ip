PKGNAME := self-ip
GIT_VERSION :=  $(shell git tag --list | tail -1)
include .env

build:
	go build -o ${PKGNAME}

run: build 
	SELF_IP_API_KEY='$(SELF_IP_API_KEY)' ./"${PKGNAME}"

clean:
	rm -f "${PKGNAME}"

.PHONY: build run clean
