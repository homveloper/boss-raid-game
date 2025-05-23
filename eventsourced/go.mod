module eventsourced

go 1.22

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.3.1
	github.com/stretchr/testify v1.8.4
	github.com/yourusername/nodestorage/v2 v2.0.0
	go.mongodb.org/mongo-driver v1.12.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// 로컬 개발을 위한 replace 지시문
replace github.com/yourusername/nodestorage/v2 => ../nodestorage/v2
