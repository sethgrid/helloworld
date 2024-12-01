# Makefile

LANG=en_US.UTF-8
SHELL=/bin/bash
.SHELLFLAGS=--norc --noprofile -e -u -o pipefail -c

# Make does not run in your shell. If you have go's path set in your shell
GOLOCATION=/usr/local/go/bin

BUILD_DIR := ./bin

.PHONY: build targets

targets:
	@make -qp | awk -F: '/^[a-zA-Z0-9][^$#\/\t=]*:([^=]|$$)/ {split($$1,A,/ /);for(i in A)print A[i]}' | sort

build:
		@echo "Building helloworld..."
		mkdir -p $(BUILD_DIR)
		chmod 755 $(BUILD_DIR)
		rm $(BUILD_DIR)/helloworld || true
		export PATH=${PATH}:${GOLOCATION} && go build -ldflags="-X 'main.Version=$(shell cat VERSION)'" -o ${BUILD_DIR}/helloworld ./cmd/helloworld

kill:
		pkill helloworld

db-restart:
		docker compose stop mysql || true
		docker compose rm -f mysql || true
		docker compose up -d mysql

db-manage:
		@echo "docker compose exec mysql mysql -uroot helloworld -proot helloworld"
		@echo "(use pw: root)"
		docker compose exec mysql mysql -uroot -proot helloworld


test:
		@echo "running go test ./..."
		go test ./...

test-unitintegration:
		@echo "running go test ./..."
		go test ./... -tags=unitintegration -count=1


clean: db-restart
		@echo "Cleaning up..."
		rm -f ./bin/helloworld || true
		docker compose stop --force helloworld || true
		docker compose rm --force helloworld || true

test-integration-dirty:
		@echo "Ensure that you are running a local instance of helloworld"
		@echo "Building integration_tests..."
		export PATH=${PATH}:${GOLOCATION} && go build -o $(BUILD_DIR)/integration_tests cmd/integration_tests/main.go

		$(eval DB_ID := $(shell sudo docker ps | grep mysql | cut -d' ' -f1))
		$(eval DB_ADDR := $(shell docker inspect ${DB_ID} -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ))

		DB_ADDR="${DB_ADDR}" DB_PORT="3306" ./bin/integration_tests

test-integration: clean
		@echo "Ensure that you are running a local instance of helloworld"
		@echo "Building integration_tests..."
		export PATH=${PATH}:${GOLOCATION} && go build -o $(BUILD_DIR)/integration_tests cmd/integration_tests/main.go

		$(eval DB_ID := $(shell sudo docker ps | grep mysql | cut -d' ' -f1))
		$(eval DB_ADDR := $(shell docker inspect ${DB_ID} -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ))

		DB_ADDR="${DB_ADDR}" DB_PORT="3306" ./bin/integration_tests


test-integration-docker: clean build
		@echo "Docker integration_tests..."
			docker compose up -d mysql
				docker compose up -d helloworld
					docker compose build integration
						docker compose run --rm integration
