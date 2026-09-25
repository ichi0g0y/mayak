module github.com/local/mayak

go 1.26.4

toolchain go1.26.8

require (
	github.com/AdguardTeam/urlfilter v0.23.4
	github.com/fsnotify/fsnotify v1.8.0
	github.com/gorilla/websocket v1.5.3
	github.com/syndtr/goleveldb v1.0.0
	github.com/wailsapp/go-webview2 v1.0.19
	github.com/wailsapp/wails/v3 v3.0.0-beta.24
	golang.org/x/image v0.46.0
	golang.org/x/sys v0.48.0
	golang.org/x/text v0.42.0
)

require (
	github.com/AdguardTeam/golibs v0.35.13 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/c2h5oh/datasize v0.0.0-20231215233829-aa82cc1e6500 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
)

replace github.com/wailsapp/go-webview2 => ./third_party/go-webview2
