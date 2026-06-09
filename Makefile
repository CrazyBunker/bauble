battary-indicator:
	go build -o bin/battary-indicator$(ext) cmd/battary-indicator/battary-indicator.go

yarurf-balance-checker:
	go get github.com/sirupsen/logrus gopkg.in/yaml.v3
	go build -o bin/yarurf-balance-checker$(ext) cmd/yarurf-balance-checker/yarurf-balance-checker.go

# Run tests for yarurf-balance-checker

test-yarurf-balance-checker:
	go test ./cmd/yarurf-balance-checker -coverprofile=cover.out

release:
	git tag -a v$(v) -m "Release $(v)"
	git push origin v$(v)

all: battary-indicator yarurf-balance-checker
