
build-openwrt:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips32le GOMIPS=softfloat GOMIPS_SOFTFLOAT=1 \
	go build -ldflags '-s -w' -compiler gc \
	  -o bin/udp2mqtt_mipsle32le_openwrt udp2mqtt.go

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags '-s -w' \
	  -o bin/udp2mqtt_linux_amd64 udp2mqtt.go

build-macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	go build -ldflags '-s -w' \
	  -o bin/udp2mqtt_darwin_amd64 udp2mqtt.go

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build -ldflags '-s -w' \
	  -o bin/udp2mqtt_-windows_amd64.exe udp2mqtt.go
