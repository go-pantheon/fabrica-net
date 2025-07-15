server: stop_server build_server start_server
	tail -f bin/server.log

client: stop_client build_client start_client
	tail -f bin/client.log

build_server:
	rm -f bin/server && go build -o bin/ ./server

build_client:
	rm -f bin/client && go build -o bin/ ./client

start_server:
	nohup $(shell pwd)/bin/server 2>&1 > bin/server.log &

start_client:
	nohup $(shell pwd)/bin/client 2>&1 > bin/client.log &

stop_server:
	-pkill -f $(shell pwd)/bin/server

stop_client:
	-pkill -f $(shell pwd)/bin/client

# KCP-specific targets for different configurations
gaming: stop_all all start_gaming
	@echo "Starting KCP with gaming-optimized configuration..."

streaming: stop_all all start_streaming
	@echo "Starting KCP with streaming-optimized configuration..."

all: build_server build_client

start_gaming:
	KCP_CONFIG=gaming nohup $(shell pwd)/bin/server 2>&1 > bin/server-gaming.log &
	sleep 2
	KCP_CONFIG=gaming nohup $(shell pwd)/bin/client 2>&1 > bin/client-gaming.log &

start_streaming:
	KCP_CONFIG=streaming nohup $(shell pwd)/bin/server 2>&1 > bin/server-streaming.log &
	sleep 2
	KCP_CONFIG=streaming nohup $(shell pwd)/bin/client 2>&1 > bin/client-streaming.log &

stop_all: stop_server stop_client

# Performance monitoring
monitor:
	@echo "Monitoring KCP performance..."
	tail -f bin/*.log | grep -E "(Performance|Pool|KCP)"

# Benchmark different configurations
benchmark:
	@echo "Running KCP configuration benchmark..."
	go run benchmark.go

# Test the example
test:
	@echo "Running KCP example tests..."
	./test.sh

# Clean up logs and binaries
clean:
	rm -f bin/*

.PHONY: server client build_server build_client start_server start_client stop_server stop_client gaming streaming monitor benchmark test clean
