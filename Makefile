GOARCH=amd64

CGO_ENABLED=1

GOBUILD=go build

OUTPUT_NAME=port-forward-gui

OUTPUT=./bin

usage:
	@echo "make init"
	@echo "make build"
	@echo "make clean"

init:
	mkdir -p ${OUTPUT}
	go mod tidy

clean:
	rm -rf  ./${OUTPUT}/*

build:
	CGO_ENABLED=${CGO_ENABLED} GOOS=windows GOARCH=${GOARCH} ${GOBUILD} -o ${OUTPUT}/${OUTPUT_NAME}.exe .
	chmod +x ./${OUTPUT}/${OUTPUT_NAME}.exe

build-osx:
	CGO_ENABLED=${CGO_ENABLED} GOOS=darwin GOARCH=${GOARCH} ${GOBUILD} -o ${OUTPUT}/${OUTPUT_NAME}  .
	chmod +x ./${OUTPUT}/${OUTPUT_NAME}
	mkdir -p ./${OUTPUT}/osx && mv ./${OUTPUT}/${OUTPUT_NAME} ./${OUTPUT}/osx/${OUTPUT_NAME}

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} ${GOBUILD} -gcflags='all=-l' -ldflags='-s' -trimpath -buildvcs=false -o ${OUTPUT}/${OUTPUT_NAME}   .
	chmod +x ./${OUTPUT}/${OUTPUT_NAME}
	mkdir -p ./${OUTPUT}/linux && mv ./${OUTPUT}/${OUTPUT_NAME} ./${OUTPUT}/linux/${OUTPUT_NAME}

build-all: clean build build-osx build-linux