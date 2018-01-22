
PLUGIN_OWNER        := rchicoli
PLUGIN_NAME         := docker-log-logstash
PLUGIN_TAG          ?= development

BASE_DIR            ?= .
ROOTFS_DIR          ?= $(BASE_DIR)/plugin/rootfs
DOCKER_COMPOSE_FILE ?= $(BASE_DIR)/docker/docker-compose.yml
SCRIPTS_DIR         ?= $(BASE_DIR)/scripts
TESTS_DIR           ?= $(BASE_DIR)/tests

SHELL               := /bin/bash
DOCKER_COMPOSE      := $(shell which docker-compose)

.PHONY: all clean docker rootfs create install enable

all: clean docker rootfs create enable clean

clean:
	@echo ""
	test -d $(ROOTFS_DIR) && rm -rf $(ROOTFS_DIR) || true
	# docker rmi $(DOCKER_IMAGE):$(APP_VERSION)

docker:
	@echo ""
	docker build -t $(PLUGIN_OWNER)/$(PLUGIN_NAME):rootfs ${BASE_DIR}

rootfs:
	@echo ""
	mkdir -p ${BASE_DIR}/plugin/rootfs

	@echo ""
	docker create --name tmprootfs $(PLUGIN_OWNER)/$(PLUGIN_NAME):rootfs
	docker export tmprootfs | tar -x -C ${BASE_DIR}/plugin/rootfs
	docker rm -vf tmprootfs

create:
	@echo ""
	docker plugin rm -f $(PLUGIN_OWNER)/$(PLUGIN_NAME):$(PLUGIN_TAG) || true

	@echo ""
	docker plugin create $(PLUGIN_OWNER)/$(PLUGIN_NAME):$(PLUGIN_TAG) ${BASE_DIR}/plugin

install:
	docker plugin install $(PLUGIN_OWNER)/$(PLUGIN_NAME):$(PLUGIN_TAG) --alias logstash

enable:
	@echo ""
	docker plugin enable $(PLUGIN_OWNER)/$(PLUGIN_NAME):$(PLUGIN_TAG)

push: clean docker rootfs create enable
	@echo ""
	docker plugin push $(PLUGIN_OWNER)/$(PLUGIN_NAME):$(PLUGIN_TAG)

# binary:
# 	docker run --rm -v $(PWD):$(WORKDIR) -w $(WORKDIR) golang:1.7.1-alpine go build -ldflags '-extldflags "-static"' -o $(APP_NAME) main.go

# tag:
# 	docker tag $(DOCKER_IMAGE):$(APP_VERSION) $(DOCKER_IMAGE):latest

docker_compose:
ifeq (, $(DOCKER_COMPOSE))
	$(error "docker-compose: command not found")
endif

deploy_logstash: docker_compose

	# create and run logstash as a container
	docker-compose -f "$(DOCKER_COMPOSE_FILE)" up -d logstash

stop_logstash: docker_compose
	docker-compose -f "$(DOCKER_COMPOSE_FILE)" stop logstash

undeploy_logstash: docker_compose
	docker-compose -f "$(DOCKER_COMPOSE_FILE)" rm --stop --force logstash


deploy_webapper: deploy_logstash docker_compose
	# create a container for logging to logstash
	$(SCRIPTS_DIR)/wait-for.sh logstash 5000 docker-compose -f "$(DOCKER_COMPOSE_FILE)" up -d webapper

stop_webapper:
	# create a container for logging to logstash
	docker-compose -f "$(DOCKER_COMPOSE_FILE)" stop webapper

undeploy_webapper:
	# create a container for logging to logstash
	docker-compose -f "$(DOCKER_COMPOSE_FILE)" rm -s -f webapper

create_environment: deploy_logstash deploy_webapper

delete_environment: stop_webapper stop_logstash

acceptance_tests: create_environment
	bats $(TESTS_DIR)/main.bats
