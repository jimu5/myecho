.PHONY: all run clean help

APP = myecho

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

.PHONY: tidy
tidy:
	@go mod tidy

## test: Run unit test.
.PHONY: test
test:
	@$(MAKE) go.test

## 清理二进制文件
clean:
	@if [ -f ./bin/${APP}-linux64 ] ; then rm ./bin/${APP}-linux64; fi
	@if [ -f ./bin/${APP}-win64.exe ] ; then rm ./bin/${APP}-win64.exe; fi
	@if [ -f ./bin/${APP}-darwin64 ] ; then rm ./bin/${APP}-darwin64; fi

help:
	@echo "make - 格式化 Go 代码, 并编译生成二进制文件"
	@echo "make mac - 编译 Go 代码, 生成mac的二进制文件"
	@echo "make linux - 编译 Go 代码, 生成linux二进制文件"
	@echo "make win - 编译 Go 代码, 生成windows二进制文件"
	@echo "make tidy - 执行go mod tidy"
	@echo "make run - 直接运行 Go 代码"
	@echo "make clean - 移除编译的二进制文件"
	@echo "make all - 编译多平台的二进制文件"