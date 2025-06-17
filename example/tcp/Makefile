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
