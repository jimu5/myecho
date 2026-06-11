.PHONY: all build run fmt vet test coverage check tidy admin-test admin-build package clean help

APP = myecho
PACKAGE_OS ?= linux
PACKAGE_ARCH ?= amd64
PACKAGE_CGO_ENABLED ?= 0
DIST_DIR ?= dist
PACKAGE_NAME = ${APP}-${PACKAGE_OS}-${PACKAGE_ARCH}
PACKAGE_DIR = ${DIST_DIR}/${PACKAGE_NAME}
PACKAGE_ARCHIVE = ${DIST_DIR}/${PACKAGE_NAME}.tar.gz

## linux: 编译打包linux
.PHONY: linux
linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ./bin/${APP}-linux64 .
	chmod 777 ./bin/${APP}-linux64

## win: 编译打包win
.PHONY: win
win:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o ./bin/${APP}-win64.exe .
	chmod 777 ./bin/${APP}-win64.exe

## mac: 编译打包mac
.PHONY: mac
mac:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build  -o ./bin/${APP}-darwin64 .
	chmod 777 ./bin/${APP}-darwin64

build:
	@go build -o ${APP}

## 编译win，linux，mac平台
.PHONY: all
all:win linux mac

run:
	@go run ./

.PHONY: fmt
fmt:
	@gofmt -w $$(find . -path './fe' -prune -o -name '*.go' -print)

.PHONY: vet
vet:
	@go vet ./...

.PHONY: tidy
tidy:
	@go mod tidy

## test: Run unit test.
.PHONY: test
test:
	@go test ./...

coverage:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out

check: vet test

admin-test:
	@cd fe/myecho-admin && npm test -- --watchAll=false

admin-build:
	@cd fe/myecho-admin && npm run build
	@rm -rf static/admin
	@mkdir -p static
	@cp -R fe/myecho-admin/build static/admin

package: admin-build
	@rm -rf ${PACKAGE_DIR} ${PACKAGE_ARCHIVE}
	@mkdir -p ${PACKAGE_DIR}/static ${PACKAGE_DIR}/storage
	@CGO_ENABLED=${PACKAGE_CGO_ENABLED} GOOS=${PACKAGE_OS} GOARCH=${PACKAGE_ARCH} go build -o ${PACKAGE_DIR}/${APP} .
	@cp config.example.yaml ${PACKAGE_DIR}/config.example.yaml
	@cp -R views ${PACKAGE_DIR}/views
	@cp -R static/admin ${PACKAGE_DIR}/static/admin
	@tar -czf ${PACKAGE_ARCHIVE} -C ${DIST_DIR} ${PACKAGE_NAME}
	@echo "package created: ${PACKAGE_ARCHIVE}"

## 清理二进制文件
clean:
	@if [ -f ./bin/${APP}-linux64 ] ; then rm ./bin/${APP}-linux64; fi
	@if [ -f ./bin/${APP}-win64.exe ] ; then rm ./bin/${APP}-win64.exe; fi
	@if [ -f ./bin/${APP}-darwin64 ] ; then rm ./bin/${APP}-darwin64; fi

help:
	@echo "make build - 编译当前平台二进制"
	@echo "make - 格式化 Go 代码, 并编译生成二进制文件"
	@echo "make mac - 编译 Go 代码, 生成mac的二进制文件"
	@echo "make linux - 编译 Go 代码, 生成linux二进制文件"
	@echo "make win - 编译 Go 代码, 生成windows二进制文件"
	@echo "make fmt - 格式化 Go 代码"
	@echo "make vet - 执行 go vet"
	@echo "make test - 执行 Go 单元测试"
	@echo "make coverage - 生成 Go 覆盖率报告"
	@echo "make check - 执行 Go vet 和测试"
	@echo "make admin-test - 执行后台前端测试"
	@echo "make admin-build - 构建后台前端到 static/admin"
	@echo "make package - 一键构建后台前端、Linux 后端并打包到 dist/"
	@echo "make tidy - 执行go mod tidy"
	@echo "make run - 直接运行 Go 代码"
	@echo "make clean - 移除编译的二进制文件"
	@echo "make all - 编译多平台的二进制文件"
