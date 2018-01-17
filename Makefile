
PLUGIN_OWNER = rchicoli
PLUGIN_NAME  = docker-log-logstash
PLUGIN_TAG  ?= development

BASE_DIR    ?= .
ROOTFS_DIR   = ./plugin/rootfs

.PHONY: all clean docker rootfs create install enable

all: clean docker rootfs create enable clean

clean:
	@echo "# clean: removes rootfs directory"
	test -d ${ROOTFS_DIR} && rm -rf ${ROOTFS_DIR} || true
# 	docker rmi $(DOCKER_IMAGE):$(APP_VERSION)

docker:
	@echo ""
	@echo "# docker: docker build: rootfs image with binary"
	docker build -t ${PLUGIN_OWNER}/${PLUGIN_NAME}:rootfs ${BASE_DIR}

rootfs:
	@echo ""
	@echo "# rootfs: creates rootfs directory in ./plugin/rootfs"
	mkdir -p ${BASE_DIR}/plugin/rootfs

	@echo ""
	docker create --name tmprootfs ${PLUGIN_OWNER}/${PLUGIN_NAME}:rootfs
	docker export tmprootfs | tar -x -C ${BASE_DIR}/plugin/rootfs
	docker rm -vf tmprootfs

create:
	@echo ""
	@echo "# create: removes existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	docker plugin rm -f ${PLUGIN_OWNER}/${PLUGIN_NAME}:${PLUGIN_TAG} || true

	@echo ""
	@echo "# create: creates a docker new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	docker plugin create ${PLUGIN_OWNER}/${PLUGIN_NAME}:${PLUGIN_TAG} ${BASE_DIR}/plugin

install:
	docker plugin install ${PLUGIN_OWNER}/${PLUGIN_NAME}:${PLUGIN_TAG} --alias logstash

enable:
	@echo ""
	@echo "# enable: enables the docker plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	docker plugin enable ${PLUGIN_OWNER}/${PLUGIN_NAME}:${PLUGIN_TAG}

push: clean docker rootfs create enable
	@echo ""
	@echo "# push: publish the docker plugin ${PLUGIN_NAME}:${PLUGIN_TAG}"
	docker plugin push ${PLUGIN_OWNER}/${PLUGIN_NAME}:${PLUGIN_TAG}

# binary:
# 	docker run --rm -v $(PWD):$(WORKDIR) -w $(WORKDIR) golang:1.7.1-alpine go build -ldflags '-extldflags "-static"' -o $(APP_NAME) main.go

# tag:
# 	docker tag $(DOCKER_IMAGE):$(APP_VERSION) $(DOCKER_IMAGE):latest

# test:
# 	go test -v
