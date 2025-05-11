module eventsync

go 1.24.1

require (
	go.mongodb.org/mongo-driver v1.13.1
	go.uber.org/zap v1.26.0
	nodestorage/v2 v2.0.0-00010101000000-000000000000
)

replace nodestorage/v2 => ../nodestorage/v2
