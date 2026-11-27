# SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
# SPDX-License-Identifier: GPL-3.0-only

PKGNAME := self-ip
GIT_VERSION :=  $(shell git describe --tags --abbrev=0)
include .env

build:
	go build -o ${PKGNAME}

test:
	rm -f ./state/banManager.gob
	go test -count=1 ./handlers

run: build
	SELF_IP_API_KEY='$(SELF_IP_API_KEY)' SELF_IP_API_KEY_HASH_ALGORITHM='SHA512' \
		SELF_IP_ENABLE_PORT_CHECKER='True' SELF_IP_ENABLE_HOSTNAME='True' ./"${PKGNAME}"

clean:
	rm -f "${PKGNAME}"

.PHONY: build run clean test
