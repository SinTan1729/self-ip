PKGNAME := self-ip
GIT_VERSION :=  $(shell git describe --tags --abbrev=0)
include .env

build:
	go build -o ${PKGNAME}

test:
	go test ./handlers

run: build
	SELF_IP_API_KEY='$(SELF_IP_API_KEY)' SELF_IP_ENABLE_PORT_CHECKER='True' SELF_IP_ENABLE_HOSTNAME='True' ./"${PKGNAME}"

clean:
	rm -f "${PKGNAME}"

.PHONY: build run clean test
