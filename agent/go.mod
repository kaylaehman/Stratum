module github.com/kaylaehman/stratum/agent

go 1.24.0

require (
	github.com/fsnotify/fsnotify v1.10.1
	github.com/kaylaehman/stratum/proto v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace github.com/kaylaehman/stratum/proto => ../proto
