


.PHONY: generate-mocks
generate-mocks:
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		vektra/mockery:v3.7.1 --config .mockery.yaml --log-level debug


.PHONY: test
test:
	go test -v -race ./... -coverprofile=coverage.txt -covermode=atomic
	go tool cover -func coverage.txt | grep total
