
clean:
	go clean

build:
	go build

docker:
	@echo "Docker Build..."
	docker build . -t crossoverjie/k8s-combat:log && docker image push crossoverjie/k8s-combat:log